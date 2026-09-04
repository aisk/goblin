# Benchmark Results

Measured 2026-09-04 on the call-path allocation work, against `8ffddef` as the
baseline. The previous round's measurements and analysis (direct-call lowering,
for-range loop lowering) are in this file's history at `8ffddef`.

Four benchmarks are new this round — `wordfreq`, `objects`, `callbacks` and
`logparse`. The six that were here before are scalar and loop dominated, and
they turned out to be blind to the cost this round removes; the new four are
shaped like ordinary scripts and see it. See [README.md](README.md).

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
built ahead of time so compilation is never timed. All ten benchmarks produce
byte-identical output across all five languages and across old/new transpiler
and interpreter, verified before timing.

## Wall-clock time (best of 5, ms)

| benchmark | go | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|---:|
| fib | 10 | 38 | 97 | 182 | **67** | 2467 |
| sieve | 12 | 72 | 135 | 680 | **476** | 3677 |
| mandelbrot | 9 | 28 | 108 | 744 | **11** | 2763 |
| nqueens | 14 | 34 | 198 | 408 | **281** | 5264 |
| matmul | 30 | 51 | 172 | 1145 | **468** | 4446 |
| hanoi | 7 | 31 | 73 | 163 | **58** | 2222 |
| wordfreq | 124 | 250 | 537 | 720 | **528** | 2739 |
| objects | 31 | 45 | 470 | 605 | **351** | 3588 |
| callbacks | 27 | 75 | 261 | 573 | **464** | 3629 |
| logparse | 87 | 325 | 1868 | 942 | **797** | 3563 |

Goblin transpile + `go build` adds a flat **205–250 ms** per program, not
included above.

### Interpreter startup, measured separately (previous round)

An empty program costs ~1 ms for a `build-exe` binary and ~5 ms for
`goblin run`; python3 9 ms, node 16 ms. The ratios below subtract these.
Anything under ~15 ms is an order of magnitude, not a measurement.

## Slowdown vs Go (startup subtracted)

| benchmark | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|
| fib | 2.8× | 12× | 22× | **8.3×** | 308× |
| sieve | 5.6× | 14× | 67× | 48× | 367× |
| mandelbrot | 1.7× | 15× | 105× | **1.4×** | 394× |
| nqueens | 1.5× | 17× | 33× | 23× | 438× |
| matmul | 1.3× | 6.1× | 41× | 17× | 159× |
| hanoi | 3.0× | 15× | 31× | **11×** | 443× |
| wordfreq | 1.9× | 4.4× | 5.8× | **4.3×** | 22× |
| objects | 1.0× | 16× | 21× | **12×** | 124× |
| callbacks | 2.4× | 10× | 23× | 19× | 145× |
| logparse | 3.6× | 22× | 11× | **9.4×** | 42× |

## What the numbers say

### 1. What changed this round

Four changes to the call path, in the runtime rather than in the generated
code, so both backends get them. No semantic change: every benchmark and every
example still produces byte-identical output on both backends, error messages
and tracebacks included, and `go test ./...` passes.

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
| wordfreq | 2299 | 524 | **−77%** | 4343 | 2763 | **−36%** |
| logparse | 2743 | 807 | **−71%** | 5366 | 3589 | **−33%** |
| objects | 980 | 351 | **−64%** | 5196 | 3620 | **−30%** |
| sieve | 868 | 470 | **−46%** | 3965 | 3673 | −7% |
| matmul | 509 | 469 | −8% | 4522 | 4471 | −1% |
| mandelbrot | 10 | 10 | 0 | 2710 | 2769 | +2% |
| fib | 66 | 66 | 0 | 2378 | 2472 | +4% |
| hanoi | 57 | 57 | 0 | 2168 | 2234 | +3% |
| nqueens | 277 | 280 | +1% | 5220 | 5313 | +2% |
| callbacks | 429 | 467 | +9% | 3573 | 3647 | +2% |

The split is by shape, not by size. A benchmark that calls a built-in method
or passes a keyword argument in its hot loop gets 2.8–4.4× compiled and about
1.5× interpreted. Of the original six only sieve does (`flags.push` while
filling the array), and it is the only one of the six that moves.

**Callbacks got 9% slower, and that is the cost of this round.** `CallArgs`
grew from 32 to 48 bytes when its keyword field went from a map header to a
slice header, and that struct is copied on every call. Programs whose inner
loop is a call through a function value — a callback handed to `map`, a
function stored in a dict — get nothing back from the other three changes and
pay this. Isolating it, by padding the old `CallArgs` to 48 bytes and changing
nothing else, reproduces the whole effect, so it is the struct size and not
the new code. Putting the keyword field behind a pointer would win it back and
cost an allocation on every keyword-carrying call instead, which is the more
common shape; callbacks is the price of that choice, and it is in the table on
purpose.

### 2. Where the compiled backend stands

| benchmark | build-exe vs CPython |
|---|---|
| mandelbrot | **68× faster** |
| hanoi | **2.8× faster** |
| fib | **2.7× faster** |
| matmul | **2.4× faster** |
| objects | **1.7× faster** |
| nqueens | **1.5× faster** |
| sieve | **1.4× faster** |
| wordfreq | **1.4× faster** |
| callbacks | **1.2× faster** |
| logparse | **1.2× faster** |

Sieve was 1.2× *slower* than CPython last round, and the four new benchmarks
were all slower than CPython before this round's work. The compiled backend
now beats CPython on all ten, though on the script-shaped four the margin is
thin.

The benchmarks fall into three groups:

- **fib / hanoi / mandelbrot / nqueens — 8–23× off Go.** Scalar- and
  call-dominated. What remains is that arguments and returned values are still
  boxed `object.Object`s and every arithmetic step on them is an interface
  call.
- **sieve / matmul — 17–48× off Go.** Collection-dominated. Their inner loops
  read and write list elements, which are still boxed; nothing so far touches
  that.
- **wordfreq / objects / callbacks / logparse — 4.3–19× off Go.** The shapes
  this round targeted. They are now the closest group to Go, which is what a
  round of removing fixed per-call costs should look like.

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

**Calling a function value still allocates its arguments — this is what
callbacks measures.** After this round the only fixed allocation left on the
call path is the argument slice for a call whose callee is a value rather than
a name. Escape analysis cannot see through the indirection, and
`List.mapMethod` builds a fresh one-element slice per element.

**Integer results are still boxed.** Go's runtime interns 0–255, so only
values outside that range allocate. A pre-boxed small-integer table would take
most of it, and a value-type `object.Object` would take all of it — a
micro-benchmark puts scalar and string boxing at 10–30× between the two
representations — but that is a rewrite of the runtime, both backends, and
every extension module.

**The interpreter's scopes are maps.** `goblin run` still spends a large share
in `mapaccess2_faststr`. Resolving identifiers to `(depth, index)` slots at
parse time is the fix, with the REPL kept on the map-based environment.

### 4. `build-exe` vs `goblin run`

| benchmark | speedup from compiling |
|---|---:|
| mandelbrot | **251×** |
| hanoi | 38× |
| fib | 37× |
| nqueens | 19× |
| objects | 10× |
| matmul | 9.5× |
| callbacks | 7.8× |
| sieve | 7.7× |
| wordfreq | 5.2× |
| logparse | 4.5× |

The gap is narrowest exactly where the interpreter spends its time in the same
shared runtime the compiled program calls: string and dict work in `object/`
costs both backends the same, so compiling only removes the tree walk.

### 5. Cross-language sanity check

Go and Node are within 1.0–5.6× of each other, Lua lands 4.4–22× behind Go,
and CPython 5.8–105× — consistent with what these benchmarks report elsewhere,
so the harness is not skewing anything. The script-shaped four compress every
column, because there the work is inside each language's string and hash-table
implementation rather than in its interpreter loop.

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
