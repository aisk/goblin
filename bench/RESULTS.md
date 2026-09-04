# Benchmark Results

Measured 2026-09-04 on the call-path allocation work (`e414b64`), against
`8ffddef` as the baseline. The previous round's measurements and analysis
(direct-call lowering, for-range loop lowering) are in this file's history at
`8ffddef`.

## Environment

```
Linux 6.18.35.2-microsoft-standard-WSL2 x86_64 (WSL2)
AMD Ryzen 7 5700X 8-Core Processor
Python 3.14.6
Lua 5.5.1
Node v25.3.0
Go 1.26.7
```

Method: `./run.sh 5` — best of 5 wall-clock runs per implementation, binaries
built ahead of time so compilation is never timed. All six benchmarks produce
byte-identical output across all five languages and across old/new transpiler
and interpreter, verified before timing.

## Wall-clock time (best of 5, ms)

| benchmark | go | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|---:|
| fib | 10 | 37 | 96 | 180 | **66** | 2460 |
| sieve | 12 | 69 | 132 | 674 | **461** | 3635 |
| mandelbrot | 9 | 28 | 107 | 702 | **11** | 2728 |
| nqueens | 14 | 34 | 201 | 408 | **280** | 5254 |
| matmul | 30 | 50 | 173 | 1120 | **470** | 4444 |
| hanoi | 7 | 31 | 74 | 164 | **58** | 2238 |

Goblin transpile + `go build` adds a flat **202–222 ms** per program, not
included above.

### Interpreter startup, measured separately (previous round)

An empty program costs ~1 ms for a `build-exe` binary and ~5 ms for
`goblin run`; python3 9 ms, node 16 ms. The ratios below subtract these.
Anything under ~15 ms is an order of magnitude, not a measurement.

## Slowdown vs Go (startup subtracted)

| benchmark | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|
| fib | 2.6× | 12× | 21× | **8.1×** | 307× |
| sieve | 5.3× | 13× | 67× | 46× | 363× |
| mandelbrot | 1.7× | 15× | 99× | **1.4×** | 389× |
| nqueens | 1.5× | 17× | 33× | 23× | 437× |
| matmul | 1.2× | 6.2× | 40× | 17× | 159× |
| hanoi | 3.0× | 15× | 31× | **11×** | 447× |

## What the numbers say

### 1. What changed this round

Four changes to the call path, in the runtime rather than in the generated
code, so both backends get them. No semantic change: all six benchmarks and
every example still produce byte-identical output on both backends, error
messages and tracebacks included, and `go test ./...` passes.

**The per-call maps are gone.** A call carrying keyword arguments allocated a
`map[string]Object` to hold them, argument parsing allocated a second map to
track which ones it had consumed, and `BindArguments` allocated a third to
return its result. Keyword arguments are now an ordered slice scanned
linearly, consumed ones are marked in a bitmap, and binding returns values by
parameter index. A generated type constructor also gained a positional fast
path, so `Vec(1, 2)` skips binding altogether.

**Methods no longer materialise a bound function.** `xs.push(v)` used to ask
`GetAttr` for the method, which had to allocate an `object.Function` wrapper
so that `object.Call` could invoke it, and then threw the wrapper away.
`object.CallMethod` dispatches on the receiver's concrete type and calls the
Go method directly; anything that is not a method falls back to the old
`GetAttr` path, so a field holding a function, `constructor`, `attributes` and
unknown-attribute errors all behave as before.

**Argument lists stay on the stack.** The two above only pay off if the
argument slice at the call site is not heap-allocated anyway. Three things
were forcing it onto the heap: the transpiler built keyword arguments by
mutating a local instead of writing one literal, `ArgParser` returned an
`*Error` built from its own fields, and every path out of `CallMethod` led to
a function value. The first two are fixed outright; the paths that must go
through something opaque hand over a copy, so they pay for it alone.

**Built-ins are called directly.** `print(x)` and `Str(i)` went through a map
lookup on the builtins module and then `object.Call` on a function value; the
transpiler now emits the Go function.

A/B against `8ffddef`, both binaries benchmarked back-to-back in one session:

| benchmark | exe old | exe new | change | run old | run new | change |
|---|---:|---:|---:|---:|---:|---:|
| sieve | 862 | 493 | **−43%** | 3964 | 3715 | −6% |
| matmul | 513 | 472 | −8% | 4471 | 4487 | 0 |
| fib | 66 | 66 | 0 | 2381 | 2472 | +4% |
| hanoi | 57 | 58 | +2% | 2198 | 2237 | +2% |
| nqueens | 274 | 282 | +3% | 5218 | 5282 | +1% |
| mandelbrot | 10 | 10 | 0 | 2738 | 2743 | 0 |

**These six benchmarks are the wrong shape to show this round's win, and the
table says so.** Only sieve calls a method in its hot loop (`flags.push`), and
it is the only one that moves. The others are scalar and call dominated, and
they pay a small tax instead: `CallArgs` grew from 32 to 48 bytes when its
keyword field went from a map header to a slice header, and that struct is
copied on every call. Isolating it — padding the old `CallArgs` to 48 bytes
and changing nothing else — reproduces the whole 2–4%, so it is the struct
size and not the new code.

Method- and keyword-heavy programs are where the work lands, and none of them
are in `bench/`. Four throwaway macro programs (word counting, user types with
dunder operators, closures over `map`/`filter`/`reduce`, log line parsing)
measured **2.7–3.6× faster compiled** and **1.4–1.6× faster interpreted**,
except the closure one, which is the pure-callback shape and pays the same
2–8% `CallArgs` tax as fib. That is a favourable trade, but it is worth
knowing the tax exists.

### 2. Where the compiled backend stands

| benchmark | build-exe vs CPython |
|---|---|
| mandelbrot | **64× faster** |
| hanoi | **2.8× faster** |
| fib | **2.7× faster** |
| matmul | **2.4× faster** |
| sieve | **1.5× faster** |
| nqueens | **1.5× faster** |

Sieve was 1.2× *slower* than CPython last round. The compiled backend now
beats CPython on all six.

The benchmarks still fall into two groups:

- **fib / hanoi / mandelbrot / nqueens — 8–23× off Go.** Scalar- and
  call-dominated. What remains is that arguments and returned values are still
  boxed `object.Object`s and every arithmetic step on them is an interface
  call.
- **sieve / matmul — 17–46× off Go.** Collection-dominated. Their inner loops
  read and write list elements, which are still boxed; nothing so far touches
  that.

### 3. What is left

**Collections are still boxed — worth ~11× on matmul.** An earlier round
measured matmul 240×240 at 38 ms with elements unboxed on read versus 431 ms
fully boxed (fully native Go: 27.7 ms). Getting it means emitting
`if x, ok := elem.(object.Integer); ok { … } else { … }` guards around
collection reads inside hot loops. Unlike everything shipped so far that is
speculative — element types are not knowable statically — and roughly doubles
the size of generated inner loops. This is the single biggest lever left for
sieve and matmul.

**Arguments and returns are still boxed.** Direct calls removed the CallArgs
machinery, but a lowered `fib(n - 1)` still boxes `n - 1` into an
`object.Object` and the callee still computes on it dynamically. Removing that
needs per-parameter type information: either interprocedural inference or
optional type annotations seeding the existing specialisation pass.

**Calling a function value still allocates its arguments.** After this round
the only fixed allocation left on the call path is the argument slice for a
call whose callee is a value rather than a name — a callback passed to
`map`/`filter`/`reduce`, a function stored in a dict, a closure. Escape
analysis cannot see through the indirection, and `List.mapMethod` builds a
fresh one-element slice per element.

**Integer results are still boxed.** Go's runtime interns 0–255, so only
values outside that range allocate; they were 1–16% of allocations in the
macro programs. A pre-boxed small-integer table would take most of it, and a
value-type `object.Object` would take all of it — a micro-benchmark puts
scalar and string boxing at 10–30× between the two representations — but that
is a rewrite of the runtime, both backends, and every extension module.

**The interpreter's scopes are maps.** `goblin run` still spends a large share
in `mapaccess2_faststr`. Resolving identifiers to `(depth, index)` slots at
parse time is the fix, with the REPL kept on the map-based environment.

### 4. `build-exe` vs `goblin run`

| benchmark | speedup from compiling |
|---|---:|
| mandelbrot | **248×** |
| hanoi | 39× |
| fib | 37× |
| nqueens | 19× |
| matmul | 9.5× |
| sieve | 7.9× |

Sieve was 4.9× last round; the compiled backend pulled ahead there without the
interpreter losing anything.

### 5. Cross-language sanity check

Go and Node are within 1.2–5.3× of each other, Lua lands 6.2–17× behind Go,
and CPython 21–99× — consistent with what these benchmarks report elsewhere,
so the harness is not skewing anything.

## Reproducing

```sh
cd bench && ./run.sh 5
```

Comparing two commits: run the A/B back-to-back in one session rather than
diffing two `run.sh` outputs taken minutes apart — machine drift between runs
is larger than several of the effects measured here. Note also that a
`build-exe` binary built from a checkout resolves `github.com/aisk/goblin`
through a `replace` pointing at the enclosing repository, so the old binary
has to be run from the old checkout or it will link the new runtime.

See [README.md](README.md) for per-benchmark parameters and the output-equality
check.
