# Benchmark Results

Measured 2026-07-26 on Goblin at commit `af81d8f`.

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
| fib | 10 | 37 | 105 | 184 | 213 | 2352 |
| sieve | 12 | 71 | 154 | 685 | 882 | 3562 |
| mandelbrot | 10 | 29 | 98 | 734 | 465 | 2810 |
| nqueens | 14 | 34 | 218 | 414 | 470 | 4981 |
| matmul | 30 | 50 | 180 | 1165 | 540 | 3898 |
| hanoi | 11 | 37 | 88 | 162 | 196 | 2063 |

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

Numbers under ~15 ms should be read as an order of magnitude, not a
measurement — run-to-run drift on this machine is a few ms, which is most of
Go's column.

## Slowdown vs Go (startup subtracted)

| benchmark | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|
| fib | 2.6× | 13× | 22× | **26×** | 293× |
| sieve | 5.5× | 15× | 68× | **88×** | 356× |
| mandelbrot | 1.6× | 12× | 91× | **58×** | 351× |
| nqueens | 1.5× | 18× | 34× | **39×** | 415× |
| matmul | 1.2× | 6.4× | 41× | **19×** | 139× |
| hanoi | 2.3× | 9.7× | 17× | **22×** | 229× |

Goblin transpile + `go build` adds a flat **216–236 ms** per program, not
included above.

## What the numbers say

### 1. The compiled backend is now within ~2× of CPython everywhere

| benchmark | build-exe vs CPython |
|---|---|
| matmul | **2.1× faster** |
| mandelbrot | **1.6× faster** |
| nqueens | 1.2× slower |
| fib | 1.2× slower |
| hanoi | 1.3× slower |
| sieve | 1.3× slower |

This is the result of the three commits described below. At `02b0475` the same
column read *7.3× slower* on fib and *8.3× slower* on hanoi; call-heavy code
was in a different league from loop-heavy code. It no longer is:

- one `matmul` inner iteration (two 2-D index reads, a multiply, an add): **39 ns**
- one `fib` call (1 argument): **30 ns**
- one `hanoi` call (4 arguments): **46 ns**

A function call used to cost 5–8× a loop-body iteration. It now costs about the
same, which is what you would expect from a runtime where both are ultimately
interface dispatch over boxed values.

### 2. What changed, and what each change bought

Three commits, ~90 lines total, no semantic change — all six benchmarks still
produce byte-identical output on both backends and `go test ./...` passes at
every one of them.

**`24340b1`** — two allocation sites removed:
- `object.BindArguments` fast path. Every call used to allocate three maps:
  `bound`, `index` (parameter name → position, though `params` is already
  ordered) and `kwExtras` (allocated even with zero keyword arguments).
- Lazy `Environment.vars`. `evalBlock` allocated a map per scope, and a loop
  body is a fresh scope on *every iteration*, so matmul's inner loop created
  13.8 M maps that mostly stayed empty.

**`e2059ee`** — `BindArgumentsInto`. The interpreter used to build a `bound`
map and immediately iterate it to `Define` every entry into the new
`Environment` — a second map. Arguments now go straight into the scope.

**`af81d8f`** — the same idea in generated code. The transpiler emitted
`BindArguments(...)` followed by `var n object.Object = bound["n"]`, i.e. it
built a map only to read it back with a key known at transpile time. Parameter
names, count and position are static; the *call shape* is not (functions are
first-class), so the check is emitted into the generated code and
`BindArguments` remains the fallback, keeping diagnostics identical:

```go
var n object.Object
if len(_callArgs_1.Keyword) == 0 && len(_callArgs_1.Positional) == 1 {
    n = _callArgs_1.Positional[0]
} else {
    _bound_2, _err_3 := object.BindArguments("fib", []string{"n"}, "", "", _callArgs_1)
    ...
}
```

Effect of `e2059ee`+`af81d8f`, A/B measured back-to-back in one session (the
only way these are comparable — see the drift note above):

| benchmark | exe before | exe after | Δ | run before | run after | Δ |
|---|---:|---:|---:|---:|---:|---:|
| fib | 1015 | 200 | **−80%** | 3418 | 2224 | **−34%** |
| hanoi | 907 | 193 | **−78%** | 3029 | 2075 | **−31%** |
| nqueens | 783 | 457 | **−41%** | 5479 | 4975 | −9% |
| matmul | 535 | 536 | 0% | 3938 | 3907 | 0% |
| sieve | 827 | 825 | 0% | 3445 | 3471 | 0% |
| mandelbrot | 440 | 440 | 0% | 2613 | 2643 | +1% |

The loop-only benchmarks are flat to the millisecond, which is the expected
signature: they never call a function. −80% is larger than the map allocation
alone accounts for — with the map gone the fast path allocates nothing at all,
so Go's escape analysis and inliner can work on the rest of the prologue.

### 3. What the profiler says is left

**matmul — variable lookup is half the interpreter's runtime.**

| symbol | flat | cum |
|---|---:|---:|
| `runtime.mapaccess2_faststr` | 25.2% | 48.7% |
| `aeshashbody` (string hashing) | 8.8% | 8.8% |
| `object.(*List).Index` | 5.3% | 5.3% |

`Environment` is a `map[string]Object` per scope, so every variable read is a
string hash plus a hash-table probe, repeated up the scope chain. Note this is
an *interpreter-only* problem: the transpiler emits real Go locals and never
constructs an `Environment` at all, which is most of why `build-exe` beats
`goblin run` by 7.2× on matmul.

The fix is resolving identifiers to `(depth, index)` slots at compile time and
indexing a slice at runtime. That needs a new resolve pass plus handling for
closure capture and the REPL's incremental scopes, so it is not a small change
— but it is the single largest recoverable cost left in the interpreter.

**fib — the callee scope is now the whole cost.**

| symbol | cum |
|---|---:|
| `runtime.mallocgc` | 35.5% |
| `object.BindArgumentsInto` | 29.6% |
| ↳ `interpreter.(*Environment).Define` | 28.6% |
| `runtime.mapassign_faststr` | 16.7% |

`BindArgumentsInto` itself is nearly free now; essentially all of its time is
`Define` writing into the scope's map. Slot-based scopes would remove this too
— same fix as above, which is why it is worth doing once rather than patching
the two paths separately.

### 4. `build-exe` is worth 4.0–11.0× over `goblin run`

| benchmark | speedup from compiling |
|---|---:|
| fib | 11.0× |
| hanoi | 10.5× |
| nqueens | 10.6× |
| matmul | 7.2× |
| mandelbrot | 6.0× |
| sieve | 4.0× |

The ordering inverted after these commits. Previously the gain was *smallest*
on call-heavy code, because per-call binding cost was paid by both backends;
now that the compiled backend does no per-call allocation, calls are exactly
where compiling pays best.

### 5. The tree-walking interpreter

`goblin run` sits 3.4–3.9× behind CPython on the loop-heavy benchmarks
(matmul, mandelbrot), 5.3× on sieve, and 12–14× on the call-heavy ones. For a
tree-walking interpreter with no bytecode compilation step, within 4× of
CPython on numeric loops is a good result; the remaining call-heavy gap is the
scope-map cost from §3.

### 6. Cross-language sanity check

Go and Node are within 1.2–5.5× of each other (JIT doing its job), Lua 5.4
lands 6.4–18× behind Go, and CPython 17–91× — all consistent with what these
benchmarks report elsewhere, which suggests the harness itself is not skewing
anything. CPython's worst showing is `mandelbrot` (91×), a pure float loop with
no library escape hatch; its best is `hanoi` (17×), where the work per bytecode
op is highest.

## Reproducing

```sh
cd bench && ./run.sh 5
```

Comparing two commits: run the A/B back-to-back in one session rather than
diffing two `run.sh` outputs taken minutes apart — machine drift between runs
is larger than several of the effects measured here.

See [README.md](README.md) for per-benchmark parameters and the output-equality
check.
