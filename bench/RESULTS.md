# Benchmark Results

Measured 2026-07-26 on Goblin at commit `0728155`.

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
| fib | 10 | 38 | 107 | 185 | 208 | 2304 |
| sieve | 13 | 80 | 173 | 769 | 734 | 3631 |
| mandelbrot | 10 | 31 | 99 | 732 | **12** | 2751 |
| nqueens | 14 | 37 | 223 | 422 | 480 | 5258 |
| matmul | 31 | 54 | 187 | 1219 | 546 | 4410 |
| hanoi | 7 | 34 | 87 | 173 | 206 | 2270 |

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

This matters at the fast end of the table: **almost half of node's 38 ms on
`fib` is V8 booting**, not computing. The ratios below subtract startup.

Anything under ~15 ms is an order of magnitude, not a measurement. Go's `hanoi`
came out at 7 ms here and 11 ms in the previous run with identical code, and
that 4 ms swing moves its ratio column by nearly 2×.

## Slowdown vs Go (startup subtracted)

| benchmark | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|
| fib | 2.8× | 13× | 22× | 26× | 287× |
| sieve | 5.8× | 16× | 69× | 67× | 330× |
| mandelbrot | 1.9× | 12× | 90× | **1.4×** | 343× |
| nqueens | 1.8× | 19× | 34× | 40× | 438× |
| matmul | 1.3× | 6.4× | 42× | 19× | 152× |
| hanoi | 3.6× | 17× | 33× | 41× | 453× |

Goblin transpile + `go build` adds a flat **200–229 ms** per program, not
included above.

## What the numbers say

### 1. Where the compiled backend stands

| benchmark | build-exe vs CPython |
|---|---|
| mandelbrot | **66× faster** |
| matmul | **2.2× faster** |
| sieve | 1.04× faster |
| fib | 1.2× slower |
| nqueens | 1.2× slower |
| hanoi | 1.3× slower |

At `02b0475` this column read *7.3× slower* on fib and *8.3× slower* on hanoi,
and mandelbrot was 48× behind native Go. The benchmarks now fall into three
groups, and the group a benchmark lands in says exactly which optimisation it
did or did not benefit from:

- **mandelbrot — 1.4× off native Go.** Its hot loop is nothing but local float
  scalars, so type specialisation compiles it to ordinary Go arithmetic.
- **fib / hanoi / nqueens — 26–41× off Go.** Call-heavy. They got the call-path
  work but nothing from specialisation, because their hot variables are
  *parameters*, which cannot be typed statically.
- **sieve / matmul — 19–67× off Go.** Loop-heavy but collection-heavy. Their
  inner loops read and write list elements, which are still boxed.

### 2. What changed, and what each change bought

Four commits. No semantic change: all six benchmarks still produce
byte-identical output on both backends, and `go test ./...` passes at each one.

**`24340b1`** — two allocation sites removed. `object.BindArguments` allocated
three maps per call (`bound`, `index`, and `kwExtras` even with zero keyword
arguments); and `evalBlock` allocated a scope map per block, which for a loop
body means *per iteration* — matmul's inner loop was creating 13.8 M maps that
mostly stayed empty.

**`e2059ee`** — `BindArgumentsInto`. The interpreter used to build a `bound` map
and immediately iterate it to `Define` every entry into the new `Environment` —
a second map. Arguments now go straight into the scope.

**`af81d8f`** — the same idea in generated code. The transpiler emitted
`BindArguments(...)` followed by `var n object.Object = bound["n"]`: a map built
only to be read back with a key known at transpile time.

**`2b183f7`** — local scalar type specialisation. A local variable whose type is
provable across every assignment in its function body is declared as a native Go
`int64`/`float64`/`bool` instead of an `object.Object`, with boxing inserted only
at the boundaries (calls, collections, `print`). mandelbrot's inner loop becomes:

```go
for (iter < max_iter) && (((zr * zr) + (zi * zi)) <= 4.0) {
```

The pass is **not speculative** — no runtime guards, no bail-out path. Anything
it cannot prove stays boxed and is generated exactly as before. Parameters are
never specialised: a caller can pass anything.

Effects, each measured A/B back-to-back in one session (the only way these are
comparable — see the drift note above):

| benchmark | `e2059ee`+`af81d8f` (calls) | `2b183f7` (specialisation) |
|---|---:|---:|
| mandelbrot | 0% | **−97%** (437 → 11 ms) |
| fib | **−80%** (1015 → 200 ms) | 0% |
| hanoi | **−78%** (907 → 193 ms) | 0% |
| nqueens | **−41%** (783 → 457 ms) | +2% |
| sieve | 0% | −12% |
| matmul | 0% | −7% |

The two are almost perfectly disjoint, which is the clearest evidence that each
one targets what it was meant to.

### 3. What is left

**Collections are still boxed — worth ~11× on matmul.** A microbenchmark of
matmul 240×240 under three value representations, using the real `object` types:

| | time |
|---|---:|
| everything boxed (what the transpiler emitted before `2b183f7`) | 431 ms |
| counters unboxed **and** list elements unboxed on read | 38 ms |
| fully native Go | 27.7 ms |

Specialising scalars alone — what `2b183f7` does — only bought matmul 7%, because
its inner loop is `ai[k] * b[k][j]`: the reads return `object.Object` and the
multiply is an interface call. Getting the rest means emitting
`if x, ok := elem.(object.Integer); ok { … } else { … }` around collection reads.
Unlike `2b183f7` **that is speculative**: element types are not knowable
statically, so it means a runtime guard and two code paths, roughly doubling the
size of generated inner loops.

**Parameters cannot be specialised.** This is what leaves fib/hanoi/nqueens at
26–41× off Go. Fixing it needs either interprocedural analysis proving every
call site passes the same type, or per-type specialised function versions chosen
at the call site — both speculative, both considerably more work than `2b183f7`.

**Boxing is the floor for everything else.** Every value that is not a
specialised local is a heap object: `object.Integer` is an int64, and Go only
gives you 0–255 for free. Where that has not been eliminated it dominates —
profiling the compiled `sieve` showed `mallocgc` at 27% and GC marking
(`scanObject`, `procyield`, `tryDeferToSpanScan`) at another 40%, with the
program's own code at 4.7% flat.

**The interpreter's scopes are maps.** `goblin run` gets nothing from any of the
above. Profiles at this commit: matmul spends `mapaccess2_faststr` flat 25% /
cum 49% with `Environment.Get` cum 43%; fib spends `Environment.Define` cum 29%.
Both are the same root cause — `Environment` is a `map[string]Object` per scope —
and both would be fixed by resolving identifiers to `(depth, index)` slots at
compile time. That needs a new resolve pass plus closure-capture and REPL
handling.

### 4. `build-exe` vs `goblin run`

| benchmark | speedup from compiling |
|---|---:|
| mandelbrot | **229×** |
| fib | 11.1× |
| nqueens | 11.0× |
| hanoi | 11.0× |
| matmul | 8.1× |
| sieve | 4.9× |

mandelbrot's 229× is the whole story of this round in one number: the compiled
backend can prove types and emit native code, and the tree-walking interpreter
structurally cannot.

### 5. The tree-walking interpreter

`goblin run` sits 3.6–3.8× behind CPython on the loop-heavy benchmarks
(matmul, mandelbrot), 4.8× on sieve, and 12–14× on the call-heavy ones. For a
tree-walking interpreter with no bytecode compilation step, within 4× of CPython
on numeric loops is a good result; the rest is the scope-map cost from §3.

### 6. Cross-language sanity check

Go and Node are within 1.3–5.8× of each other (JIT doing its job), Lua 5.4 lands
6.4–19× behind Go, and CPython 22–90× — all consistent with what these
benchmarks report elsewhere, which suggests the harness itself is not skewing
anything. CPython's worst showing is `mandelbrot` (90×), a pure float loop with
no library escape hatch.

## Reproducing

```sh
cd bench && ./run.sh 5
```

Comparing two commits: run the A/B back-to-back in one session rather than
diffing two `run.sh` outputs taken minutes apart — machine drift between runs is
larger than several of the effects measured here.

See [README.md](README.md) for per-benchmark parameters and the output-equality
check.
