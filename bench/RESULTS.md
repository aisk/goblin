# Benchmark Results

Measured 2026-09-05 on the compiled-backend boxing work, against `b19a02f`
as the baseline. The previous round's measurements and analysis (the
call-path allocation work) are in this file's history at `b19a02f`.

This round only touches what `build-exe` generates and the runtime helpers it
calls; `goblin run` is unchanged and its column below moves within noise. See
[README.md](README.md) for the benchmarks themselves.

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
byte-identical output across all five languages and across both Goblin
backends, verified before timing.

## Wall-clock time (best of 5, ms)

| benchmark | go | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|---:|
| fib | 10 | 39 | 99 | 187 | **12** | 2523 |
| sieve | 13 | 80 | 150 | 701 | **399** | 3705 |
| mandelbrot | 10 | 30 | 110 | 719 | **12** | 2766 |
| nqueens | 14 | 37 | 204 | 423 | **111** | 5311 |
| matmul | 31 | 53 | 180 | 1223 | **440** | 4416 |
| hanoi | 7 | 34 | 75 | 166 | **34** | 2293 |
| wordfreq | 127 | 261 | 552 | 726 | **505** | 2697 |
| objects | 32 | 48 | 483 | 621 | **262** | 3614 |
| callbacks | 29 | 77 | 269 | 583 | **357** | 3653 |
| logparse | 97 | 318 | 1959 | 974 | **743** | 3651 |

Goblin transpile + `go build` adds a flat **210–250 ms** per program, not
included above (the first build of a session pays the Go build cache).

### Interpreter startup, measured separately (previous round)

An empty program costs ~1 ms for a `build-exe` binary and ~5 ms for
`goblin run`; python3 9 ms, node 16 ms. The ratios below subtract these.
Anything under ~15 ms is an order of magnitude, not a measurement.

## Slowdown vs Go (startup subtracted)

| benchmark | node | lua | python | goblin build-exe | goblin run |
|---|---:|---:|---:|---:|---:|
| fib | 2.9× | 12× | 22× | **1.4×** | 315× |
| sieve | 5.8× | 14× | 63× | 36× | 336× |
| mandelbrot | 1.8× | 14× | 89× | **1.4×** | 345× |
| nqueens | 1.8× | 17× | 34× | **9.2×** | 442× |
| matmul | 1.3× | 6.2× | 42× | 15× | 152× |
| hanoi | 3.6× | 15× | 31× | **6.6×** | 458× |
| wordfreq | 2.0× | 4.4× | 5.7× | **4.0×** | 22× |
| objects | 1.1× | 16× | 20× | **8.7×** | 120× |
| callbacks | 2.3× | 9.9× | 21× | 13× | 135× |
| logparse | 3.2× | 21× | 10× | **7.8×** | 38× |

## What the numbers say

### 1. What changed this round

Five changes, all in the transpiler or in runtime helpers only generated code
calls. No semantic change: every benchmark and every example still produces
byte-identical output on both backends, error messages and tracebacks
included, and `go test ./...` passes.

**Closed functions get native signatures** (`transpiler/signatures.go`). A
module-level function whose every reference is a plain positional call has
every caller in view, so its parameter types are the join of what the call
sites pass and its return type the join of what the body returns. The
inference is a module-wide fixed point that resolves recursion (`fib(n - 1)`
seeds `n` from `fib(32)`), and such a function is generated as, say,
`func _direct_fib(n int64) (int64, error)` with no generic wrapper at all.
This is what the local scalar specialisation had been stopping at: the
function boundary.

**List indexing takes the index unboxed.** `xs[i]` and `xs[i] = v` with a
provably-integer `i` call `object.IndexInt` / `SetIndexInt`, which reach the
element directly for a `*List` and box for anything else. The index used to be
boxed on every access — an allocation for anything above 255.

**Constructions and `self` are direct.** `Vec(a, b)` supplying every field
positionally calls a generated `_new_Vec(a, b)` returning the struct literal
instead of `object.Call` on the constructor value. Inside a method,
`self.x`, `self.x = v` and `self.m()` are Go field access and a direct call
on the receiver instead of `GetAttr`, `SetAttr` and `CallMethod` dispatch.

**A native integer on the right of an operator stays native.** `total + i`,
`c - i`, `x == col`, `x < n` with a boxed left operand call `object.AddInt`
and friends instead of boxing the right side first; `if` and `while`
conditions built from a comparison use the comparison's Go bool directly
instead of boxing a Bool and calling `ToBool` on it.

**Callback methods reuse one argument slice.** `map`, `filter`, `reduce`,
`each`, `find`, `any`, `all` and `sort(key=...)` allocated a fresh
one-element slice per element; they now fill one slice per call.
`object.CallArgs` documents that its slices are only valid for the call, and
`spawn` copies what it hands to its goroutine.

A/B against `b19a02f`, both binaries built and benchmarked back-to-back in
one session (compiled backend only):

| benchmark | exe old | exe new | change |
|---|---:|---:|---:|
| fib | 73 | 12 | **-84%** |
| sieve | 514 | 422 | -18% |
| mandelbrot | 12 | 12 | 0 |
| nqueens | 303 | 111 | **-64%** |
| matmul | 519 | 435 | -17% |
| hanoi | 63 | 34 | **-47%** |
| wordfreq | 532 | 513 | -4% |
| objects | 365 | 261 | **-29%** |
| callbacks | 500 | 352 | **-30%** |
| logparse | 803 | 729 | -10% |

Measured step by step in the same session, each change lands where its shape
predicts: native signatures take fib 73 → 12 and hanoi 63 → 35; unboxed
indexing takes sieve 514 → 430; direct construction and `self` take objects
365 → 277; slice reuse takes callbacks 500 → 437; the native-right operators
and bool conditions take nqueens 219 → 111 and callbacks 437 → 352.

### 2. Where the compiled backend stands

| benchmark | build-exe vs CPython |
|---|---|
| mandelbrot | **60× faster** |
| fib | **16× faster** |
| hanoi | **4.9× faster** |
| nqueens | **3.8× faster** |
| matmul | **2.8× faster** |
| objects | **2.4× faster** |
| sieve | **1.8× faster** |
| callbacks | **1.6× faster** |
| wordfreq | **1.4× faster** |
| logparse | **1.3× faster** |

The benchmarks now fall into three groups:

- **fib / hanoi / mandelbrot — within 7× of Go, fib and mandelbrot within 1.5×.** Scalar- and
  call-dominated, and now fully native: fib is a Go function on `int64`.
  What is left in hanoi is a module-level `moves` counter that the function
  body mutates; a name referenced from a nested body is pinned to boxed by
  the flow-insensitive capture check, so `moves = moves + 1` still boxes.
- **nqueens / objects / callbacks / wordfreq / logparse — 4–13× off Go.**
  Mixed shapes. What remains is the boxing of every value that crosses a
  list, a field, a dict or a function value, and the integer results those
  produce.
- **sieve / matmul — 14–30× off Go.** Collection-dominated. Their inner
  loops read and write list elements, which are still boxed; nothing this
  round touches that. Sieve's remaining time is mostly building the
  4-million-element list (slice growth and GC scanning of a large pointer
  array), not the sieve loop itself.

### 3. What is left

**Collections are still boxed — the single biggest lever for sieve and
matmul.** Getting it means emitting `if x, ok := elem.(object.Integer)`
guards around element reads inside hot loops with a versioned fallback, which
roughly doubles the generated inner loop and is deliberately not done here.

**Module-level variables are boxed as soon as a function touches them.** A
package-level `var moves int64` would be as valid a target as a local, but
the capture check pins by name across all bodies. Sharing module-level
variable types across bodies through the same fixed point the signatures use
is the natural next step; hanoi and nqueens' `n` are the visible cases.

**Calling a function value still allocates its arguments — this is what
callbacks measures.** Escape analysis cannot see through `Function.Fn`, so
each `f(x)` through a value heap-allocates its one-element argument slice,
and the closure's integer results are boxed.

**Integer results are still boxed.** Go's runtime interns 0–255, so only
values outside that range allocate. A pre-boxed small-integer table and a
value-type `object.Object` were both analysed and deliberately deferred.

**The interpreter's scopes are maps.** `goblin run` still spends a large share
in `mapaccess2_faststr`. Resolving identifiers to `(depth, index)` slots at
parse time is the fix, with the REPL kept on the map-based environment.

### 4. `build-exe` vs `goblin run`

| benchmark | speedup from compiling |
|---|---:|
| mandelbrot | **230×** |
| fib | **210×** |
| hanoi | 67× |
| nqueens | 48× |
| objects | 14× |
| callbacks | 10× |
| matmul | 10× |
| sieve | 9× |
| wordfreq | 5× |
| logparse | 5× |

The gap is narrowest exactly where the interpreter spends its time in the same
shared runtime the compiled program calls: string and dict work in `object/`
costs both backends the same, so compiling only removes the tree walk.

### 5. Cross-language sanity check

Go and Node are within 1.0–6× of each other, Lua lands 4–24× behind Go, and
CPython 6–105× — consistent with what these benchmarks report elsewhere, so
the harness is not skewing anything. The script-shaped four compress every
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
