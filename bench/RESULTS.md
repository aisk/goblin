# Benchmark Results

Measured 2026-07-26 on Goblin at commit `24340b1`.

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
| fib | 10 | 37 | 105 | 180 | 1021 | 3418 |
| sieve | 12 | 69 | 164 | 680 | 824 | 3461 |
| mandelbrot | 9 | 28 | 96 | 676 | 437 | 2606 |
| nqueens | 15 | 36 | 215 | 405 | 778 | 5490 |
| matmul | 29 | 50 | 176 | 1116 | 533 | 3940 |
| hanoi | 7 | 31 | 83 | 160 | 902 | 3020 |

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

This matters at the fast end of the table: **almost half of node's 37 ms on
`fib` is V8 booting**, not computing. The ratios below subtract startup.

## Slowdown vs Go (startup subtracted)

| benchmark | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|
| fib | 2.6× | 13× | 21× | **128×** | 427× |
| sieve | 5.3× | 16× | 67× | **82×** | 346× |
| mandelbrot | 1.7× | 14× | 95× | **62×** | 372× |
| nqueens | 1.5× | 16× | 30× | **60×** | 422× |
| matmul | 1.3× | 6.5× | 41× | **20×** | 146× |
| hanoi | 3.0× | 16× | 30× | **180×** | 603× |

Goblin transpile + `go build` adds a flat **202–220 ms** per program, not
included above.

## What the numbers say

### 1. Function calls, not loops, are Goblin's bottleneck

The spread in the `build-exe` column is 9× (20× to 180× vs Go) and it splits
cleanly along one line — how many Goblin-level function calls the benchmark
makes:

| benchmark | dominated by | build-exe vs Go | build-exe vs CPython |
|---|---|---:|---|
| matmul | 13.8 M loop iterations, 0 calls | 20× | **2.1× faster** |
| mandelbrot | float loop, 0 calls | 62× | **1.5× faster** |
| nqueens | 2.0 M calls, each with a loop inside | 60× | 2.0× slower |
| sieve | array loop, 0 calls | 82× | 1.2× slower |
| fib | 7.0 M calls | 128× | 6.0× slower |
| hanoi | 4.2 M calls (4 args each) | 180× | 6.0× slower |

Per-operation cost of the compiled backend makes it concrete:

- one `matmul` inner iteration (two 2-D index reads, a multiply, an add): **38 ns**
- one `fib` call (1 argument): **145 ns**
- one `hanoi` call (4 arguments): **215 ns**

A single function call still costs 4–6× more than a whole arithmetic loop body.
On a transpiled-to-Go backend that is not where the time should be going.

### 2. What commit `24340b1` already removed

Two allocation sites were removed after the first round of measurements:

- **`object.BindArguments` fast path.** Every call used to allocate three maps:
  `bound`, `index` (parameter name → position, though `params` is already
  ordered) and `kwExtras` (allocated even with zero keyword arguments). When
  the callee has only fixed parameters and the caller passes them all
  positionally — the overwhelmingly common shape — only `bound` is built now.
- **Lazy `Environment.vars`.** `evalBlock` allocated a map per scope, and a
  loop body is a fresh scope on *every iteration*, so matmul's inner loop was
  creating 13.8 M maps that mostly stayed empty. The map is now allocated on
  first `Define`.

19 lines total, no semantic change:

| benchmark | run before | run after | Δ | exe before | exe after | Δ |
|---|---:|---:|---:|---:|---:|---:|
| fib | 4003 | 3418 | **−15%** | 1308 | 1021 | **−22%** |
| sieve | 4049 | 3461 | **−15%** | 835 | 824 | −1% |
| mandelbrot | 2608 | 2606 | 0 | 435 | 437 | 0 |
| nqueens | 5899 | 5490 | −7% | 946 | 778 | **−18%** |
| matmul | 4463 | 3940 | **−12%** | 538 | 533 | −1% |
| hanoi | 3517 | 3020 | **−14%** | 1351 | 902 | **−33%** |

`mandelbrot` gains nothing from the lazy scope because its inner loop body
declares `var tmp`, so that map is genuinely needed; the loop-only benchmarks
gain nothing on the compiled backend because they never call a function.

### 3. What the profiler says is left

CPU profiles of the interpreter at this commit (`runtime/pprof`, whole run):

**matmul — variable lookup is now half the runtime.**

| symbol | flat | cum |
|---|---:|---:|
| `runtime.mapaccess2_faststr` | 24.9% | 47.9% |
| `interpreter.(*Environment).Get` | 6.7% | 43.3% |
| `aeshashbody` (string hashing) | 7.4% | 7.4% |

`Environment` is a `map[string]Object` per scope, so every variable read is a
string hash plus a hash-table probe, repeated up the scope chain. The real fix
is resolving identifiers to `(depth, index)` slots at compile time and indexing
a slice at runtime — a new resolve pass, so not a small change, but it is the
single largest recoverable cost in the interpreter. A cheaper approximation is
a linear-scanned slice for scopes below ~8 bindings.

**fib — moving arguments into scope is still a third of the runtime.**

| symbol | cum |
|---|---:|
| `runtime.mapassign_faststr` | 22.5% |
| `object.BindArguments` | 16.5% |
| `interpreter.(*Environment).Define` | 15.6% |
| `runtime.mallocgc` | 29.1% |

The remaining waste is structural: `BindArguments` builds a `bound` map, and
the caller immediately iterates it to `Define` each entry into the new
`Environment` — a second map. A `BindArgumentsInto(define func(string, Object))`
variant would let the interpreter write straight into the scope and let the
transpiler assign straight to generated locals, taking the per-call map count
from 2 to 1 (or 0 once scopes are slot-based).

**GC tuning is nearly free.** With allocation this dominant, the Go default
`GOGC=100` is too aggressive. Measured on this commit:

| | default | `GOGC=400` | Δ |
|---|---:|---:|---:|
| `goblin run fib` | 3440 | 2658 | **−23%** |
| `goblin run hanoi` | 3041 | 2522 | −17% |
| `fib` build-exe | 1023 | 776 | **−24%** |
| `hanoi` build-exe | 911 | 753 | −17% |

`GOGC=800` adds nothing over 400. Raising the default (only when the user has
not set `GOGC`) is a one-line change; the cost is a larger resident set.

### 4. `build-exe` is worth 2.9–7.4× over `goblin run`

| benchmark | speedup from compiling |
|---|---:|
| matmul | 7.4× |
| nqueens | 7.1× |
| mandelbrot | 6.0× |
| sieve | 4.2× |
| fib | 3.3× |
| hanoi | 3.3× |

The gain is smallest where calls dominate — the shared `object` runtime and the
per-call binding cost are paid by both backends, so compiling away the tree
walk only removes the part that is not the bottleneck.

### 5. The tree-walking interpreter is respectable

`goblin run` sits 3.5–3.9× behind CPython on the loop-heavy benchmarks
(mandelbrot, matmul) and 5× on sieve. For a tree-walking interpreter with no
bytecode compilation step, being within 4× of CPython on numeric loops is a
good result; the 14–19× gap on fib/hanoi/nqueens is the same call-overhead
story as above, not a general interpreter weakness.

### 6. Cross-language sanity check

Go and Node are within 1.3–5.3× of each other (JIT doing its job), Lua 5.4
lands 6.5–16× behind Go, and CPython 21–95× — all consistent with what these
benchmarks report elsewhere, which suggests the harness itself is not skewing
anything. CPython's worst showing is `mandelbrot` (95×), a pure float loop with
no library escape hatch; its best is `fib` (21×), where the work per bytecode
op is highest.

## Reproducing

```sh
cd bench && ./run.sh 5
```

See [README.md](README.md) for per-benchmark parameters and the output-equality
check.
