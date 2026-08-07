# Benchmark Results

Measured 2026-08-08 on Goblin with direct-call lowering and for-range loop
lowering applied on top of `65531a3`. The previous round's measurements and
analysis (call-path allocations, local scalar specialisation) are in this
file's history at `65531a3`.

## Environment

```
Linux 6.18.35.2-microsoft-standard-WSL2 x86_64 (WSL2)
AMD Ryzen 7 5700X 8-Core Processor
Python 3.14.6
Lua 5.5.0
Node v25.3.0
Go 1.26.5
```

Method: `./run.sh 5` — best of 5 wall-clock runs per implementation, binaries
built ahead of time so compilation is never timed. All six benchmarks produce
byte-identical output across all five languages and across old/new transpiler
and interpreter, verified before timing.

## Wall-clock time (best of 5, ms)

| benchmark | go | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|---:|
| fib | 10 | 36 | 93 | 180 | **65** | 2516 |
| sieve | 12 | 68 | 134 | 685 | 850 | 4170 |
| mandelbrot | 9 | 28 | 101 | 690 | **11** | 2967 |
| nqueens | 14 | 34 | 210 | 408 | 296 | 5770 |
| matmul | 30 | 48 | 177 | 1148 | 505 | 4299 |
| hanoi | 7 | 31 | 76 | 161 | **58** | 2331 |

Goblin transpile + `go build` adds a flat **232–252 ms** per program, not
included above.

### Interpreter startup, measured separately (previous round)

An empty program costs ~1 ms for a `build-exe` binary and ~5 ms for
`goblin run`; python3 9 ms, node 16 ms. The ratios below subtract these.
Anything under ~15 ms is an order of magnitude, not a measurement.

## Slowdown vs Go (startup subtracted)

| benchmark | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|
| fib | 2.5× | 12× | 21× | **8.0×** | 314× |
| sieve | 5.2× | 13× | 68× | 85× | 417× |
| mandelbrot | 1.7× | 14× | 97× | **1.4×** | 423× |
| nqueens | 1.8× | 17× | 33× | 25× | 480× |
| matmul | 1.1× | 6.3× | 41× | 18× | 153× |
| hanoi | 3.0× | 15× | 30× | **11×** | 465× |

## What the numbers say

### 1. What changed this round

Two transpiler-only changes. No semantic change: all six benchmarks and every
example still produce byte-identical output on both backends, error messages
and tracebacks included, and `go test ./...` passes.

**Direct-call lowering.** A module-level function that nothing in the module
ever rebinds now also compiles to a plain Go closure with one parameter per
Goblin parameter. Call sites that pass plain positional arguments of the right
arity call it directly; the `&object.Function` wrapper survives as a thin
adapter, so keyword arguments, wrong arity, `spawn`, and the function used as
a value all behave exactly as before. What a lowered call skips per call: one
`object.Args` slice allocation, the `object.Call` type switch, and the
generated fast/slow binding preamble.

**For-range loop lowering.** `for x in range(a, b)` compiles to a native Go
counting loop instead of materialising a list of n boxed Integers to iterate
once. Bounds are evaluated once before the loop; non-Integer bounds go through
the same argument parser as the `range` builtin, so errors are identical. When
the loop variable provably stays Integer it joins the scalar specialisation
pass from the previous round and lives as a native `int64`; `range` is a
reserved name in Goblin, so the builtin can never be shadowed and the lowering
is never unsound.

A/B against `65531a3`, both binaries benchmarked back-to-back in one session:

| benchmark | old | new | change |
|---|---:|---:|---:|
| fib | 202 | 65 | **−68%** |
| hanoi | 176 | 58 | **−67%** |
| nqueens | 340 | 296 | −13% |
| sieve | 835 | 850 | ~0 |
| matmul | 504 | 505 | 0 |
| mandelbrot | 11 | 11 | 0 |

The split is exactly the one §3 of the previous round predicted. fib and hanoi
are call-dominated, so removing the per-call allocation and dispatch is worth
3×. nqueens is call-heavy but spends most of its time in list reads, so it
gets a smaller slice. sieve and matmul never cared about the call path — their
inner loops read and write boxed list elements, and that cost is untouched.
`goblin run` is unchanged by design: both lowerings are code generation only.

### 2. Where the compiled backend stands

| benchmark | build-exe vs CPython |
|---|---|
| mandelbrot | **63× faster** |
| fib | **2.7× faster** |
| hanoi | **2.7× faster** |
| matmul | **2.3× faster** |
| nqueens | **1.4× faster** |
| sieve | 1.2× slower |

Before this round fib and hanoi were 1.2–1.3× *slower* than CPython. The
compiled backend now beats CPython on five of six benchmarks, and fib at 65 ms
sits 8× off native Go — closer to Node (36 ms) than to Lua (93 ms).

The benchmarks now fall into two groups:

- **fib / hanoi / mandelbrot / nqueens — 8–25× off Go.** Scalar- and
  call-dominated. Both rounds of work land here; what remains is that
  arguments and returned values are still boxed `object.Object`s and every
  arithmetic step on them is an interface call.
- **sieve / matmul — 18–85× off Go.** Collection-dominated. Their inner loops
  read and write list elements, which are still boxed; nothing so far touches
  that.

### 3. What is left

**Collections are still boxed — worth ~11× on matmul.** The previous round
measured matmul 240×240 at 38 ms with elements unboxed on read versus 431 ms
fully boxed (fully native Go: 27.7 ms). Getting it means emitting
`if x, ok := elem.(object.Integer); ok { … } else { … }` guards around
collection reads inside hot loops. Unlike everything shipped so far that is
speculative — element types are not knowable statically — and roughly doubles
the size of generated inner loops. This is now the single biggest lever for
sieve and matmul.

**Arguments and returns are still boxed.** Direct calls removed the CallArgs
machinery, but a lowered `fib(n - 1)` still boxes `n - 1` into an
`object.Object` and the callee still computes on it dynamically. Removing that
needs per-parameter type information: either interprocedural inference or
optional type annotations seeding the existing specialisation pass. That is
the remaining gap between fib at 65 ms and mandelbrot-style native output.

**The interpreter's scopes are maps.** Unchanged this round, and unchanged in
the numbers: `goblin run` still spends 25–49% in `mapaccess2_faststr` on
matmul. Resolving identifiers to `(depth, index)` slots at parse time is the
fix, with the REPL kept on the map-based environment.

### 4. `build-exe` vs `goblin run`

| benchmark | speedup from compiling |
|---|---:|
| mandelbrot | **270×** |
| hanoi | 40× |
| fib | 39× |
| nqueens | 19.5× |
| matmul | 8.5× |
| sieve | 4.9× |

fib and hanoi were 11× last round; the compiled backend pulled ahead without
the interpreter losing anything.

### 5. Cross-language sanity check

Go and Node are within 1.1–5.2× of each other, Lua lands 6.3–17× behind Go,
and CPython 21–97× — consistent with what these benchmarks report elsewhere,
so the harness is not skewing anything. Note this round's Lua is 5.5.0 (the
previous round measured 5.4.7), which is why its column moved slightly.

## Reproducing

```sh
cd bench && ./run.sh 5
```

Comparing two commits: run the A/B back-to-back in one session rather than
diffing two `run.sh` outputs taken minutes apart — machine drift between runs
is larger than several of the effects measured here. (Today's drift moved the
old binary's nqueens from 480 ms at the July measurement to 340 ms.)

See [README.md](README.md) for per-benchmark parameters and the output-equality
check.
