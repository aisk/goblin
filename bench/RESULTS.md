# Benchmark Results

Measured 2026-07-26 on Goblin at commit `02b0475`.

## Environment

```
Linux 6.18.35.2-microsoft-standard-WSL2 x86_64 (WSL2)
AMD Ryzen 7 5700X 8-Core Processor
Python 3.14.6
Lua 5.4.7 (built from source; no system Lua on this machine)
Node v25.3.0
Go 1.26.5
```

Method: `./run.sh 5` — best of 5 wall-clock runs per implementation, binaries
built ahead of time so compilation is never timed. All six benchmarks produce
byte-identical output across all five languages, verified before timing.

## Wall-clock time (best of 5, ms)

| benchmark | go | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|---:|
| fib | 10 | 36 | 104 | 179 | 1308 | 4003 |
| sieve | 12 | 68 | 153 | 671 | 835 | 4049 |
| mandelbrot | 9 | 28 | 96 | 695 | 435 | 2608 |
| nqueens | 14 | 34 | 217 | 410 | 946 | 5899 |
| matmul | 30 | 50 | 179 | 1179 | 538 | 4463 |
| hanoi | 7 | 32 | 83 | 163 | 1351 | 3517 |

### Interpreter startup, measured separately

An empty program costs:

| runtime | startup |
|---|---:|
| goblin build-exe binary | 1 ms |
| lua | 1 ms |
| go binary | 2 ms |
| goblin run | 5 ms |
| python3 | 9 ms |
| node | 16 ms |

This matters at the fast end of the table: **almost half of node's 36 ms on
`fib` is V8 booting**, not computing. The ratios below subtract startup.

## Slowdown vs Go (startup subtracted)

| benchmark | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|
| fib | 2.5× | 13× | 21× | **163×** | 500× |
| sieve | 5.2× | 15× | 66× | **83×** | 404× |
| mandelbrot | 1.7× | 14× | 98× | **62×** | 372× |
| nqueens | 1.5× | 18× | 33× | **79×** | 491× |
| matmul | 1.2× | 6.4× | 42× | **19×** | 159× |
| hanoi | 3.2× | 16× | 31× | **270×** | 702× |

Goblin transpile + `go build` adds a flat **188–209 ms** per program, not
included above.

## What the numbers say

### 1. Function calls, not loops, are Goblin's bottleneck

The spread in the `build-exe` column is 14× (19× to 270× vs Go) and it splits
cleanly along one line — how many Goblin-level function calls the benchmark
makes:

| benchmark | dominated by | build-exe vs Go | build-exe vs CPython |
|---|---|---:|---|
| matmul | 13.8 M loop iterations, 0 calls | 19× | **2.2× faster** |
| mandelbrot | float loop, 0 calls | 62× | **1.6× faster** |
| sieve | array loop, 0 calls | 83× | 1.2× slower |
| nqueens | 2.0 M calls, each with a loop inside | 79× | 2.3× slower |
| fib | 7.0 M calls | 163× | 7.3× slower |
| hanoi | 4.2 M calls (4 args each) | 270× | 8.3× slower |

Per-operation cost of the compiled backend makes it concrete:

- one `matmul` inner iteration (two 2-D index reads, a multiply, an add): **39 ns**
- one `fib` call (1 argument): **185 ns**
- one `hanoi` call (4 arguments): **322 ns**

A single function call costs 5–8× more than a whole arithmetic loop body. On a
transpiled-to-Go backend that is not where the time should be going.

### 2. The cause is in `emitParameterBinding`

Every Goblin function becomes an `&object.Function{Fn: func(callArgs object.CallArgs) (object.Object, error)}`,
and the first thing every call does is `transpiler/transpiler.go:965`:

```go
bound, err := object.BindArguments("fib", []string{"n"}, "", "", callArgs)
```

`object.BindArguments` (`object/args.go:31`) allocates **two maps on every
call** — `bound` for the results and `index` for parameter-name lookup — then
the generated body reads each parameter back out by string key. So `fib(32)`
performs ~14 M map allocations plus 7 M string hashes purely to move one
integer into one variable.

Both facts are compile-time constants: the parameter names, their count, and
(at most call sites) the fact that all arguments are positional with no
varargs/kwargs. An obvious fix is a fast path that skips `BindArguments`
entirely when the callee has only fixed parameters and the call site passes
only positional arguments, assigning `callArgs.Positional[i]` straight to the
generated local. At minimum, `index` can be hoisted to a package-level var
since it is rebuilt identically on every call.

This should move `fib`/`hanoi` toward the 19–83× band the loop-heavy
benchmarks already occupy, i.e. a projected 3–8× win on call-heavy code.

### 3. `build-exe` is worth 2.6–8.3× over `goblin run`

| benchmark | speedup from compiling |
|---|---:|
| matmul | 8.3× |
| nqueens | 6.2× |
| mandelbrot | 6.0× |
| sieve | 4.9× |
| fib | 3.1× |
| hanoi | 2.6× |

The gain is smallest exactly where calls dominate — the shared `object`
runtime and the per-call binding cost are paid by both backends, so compiling
away the tree walk only removes the part that is not the bottleneck.

### 4. The tree-walking interpreter is respectable

`goblin run` sits 3.8× behind CPython on the loop-heavy benchmarks
(mandelbrot, matmul) and 6× on sieve. For a tree-walking interpreter with no
bytecode compilation step, being within 4× of CPython on numeric loops is a
good result; the 14–22× gap on fib/hanoi/nqueens is the same call-overhead
story as above, not a general interpreter weakness.

### 5. Cross-language sanity check

Go and Node are within 1.2–5× of each other (JIT doing its job), Lua 5.4 lands
6–18× behind Go, and CPython 21–98× — all consistent with what these
benchmarks report elsewhere, which suggests the harness itself is not skewing
anything. CPython's worst showing is `mandelbrot` (98×), a pure float loop with
no library escape hatch; its best is `fib` (21×), where the work per bytecode
op is highest.

## Reproducing

```sh
cd bench && ./run.sh 5
```

See [README.md](README.md) for per-benchmark parameters and the output-equality
check.
