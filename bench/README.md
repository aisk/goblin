# Goblin Benchmarks

Six classic compute-only microbenchmarks, each implemented in **Goblin, Go,
JavaScript, Lua and Python**. They exist to track Goblin's interpreter and
transpiler performance against well-understood reference points, and to catch
behavioural drift between the two Goblin backends — every implementation must
print byte-identical output.

Nothing here touches the filesystem, the network or the clock, so runs are
reproducible and the numbers are pure CPU.

## The benchmarks

| directory | what it measures | parameters | expected output |
|---|---|---|---|
| `fib/` | function call overhead (naive recursion) | `fib(32)` | `2178309` |
| `sieve/` | tight loops over one large flat array | limit `4_000_000` | `283146` |
| `mandelbrot/` | float-heavy inner loop | 160×80, 1000 iterations | 80 lines of ASCII |
| `nqueens/` | recursive backtracking | `N = 11` | `2680` |
| `matmul/` | nested loops over indexed 2-D lists | 240×240 | `66354048000` |
| `hanoi/` | deep recursion, mutating an outer variable | 21 disks | `2097151` |

Sizes are tuned so the Goblin interpreter takes a few seconds per benchmark.
That makes the compiled languages finish in milliseconds — their numbers
include roughly 1–3 ms of process startup, which matters when reading the fast
end of the table.

## Running

```sh
./run.sh          # best of 5 runs
./run.sh 11       # best of 11
```

Runtimes are taken from `$PATH`; override individually if needed:

```sh
GOBLIN=./goblin LUA=/opt/lua/bin/lua ./run.sh
```

The script builds the Go and `goblin build-exe` binaries first so compilation
is never part of a measured run, then reports the **minimum** wall-clock time
of N runs — the statistic least affected by scheduler noise. Goblin's
transpile+compile time is reported separately at the end. A runtime that is not
installed is skipped and its column shows `-`.

Latest measurements: [RESULTS.md](RESULTS.md).

## Ground rules for the implementations

- **Same algorithm, idiomatic code.** Each version uses its language's natural
  counting loop and array type. Nothing is hand-optimised beyond what an
  ordinary author would write, and nothing uses a library that would do the
  work for it (no `numpy`, no `bitarray`).
- **Same output.** Every implementation prints exactly the same bytes, which is
  what makes the cross-language check meaningful.
- **Goblin uses `while` loops** rather than `for x in range(...)`, because
  `range()` materialises a real list. That is the idiomatic way to write a hot
  counting loop in Goblin today.
- Goblin has no `%` operator and no compound assignment (`+=`), so the
  algorithms are written to avoid both; the other languages follow the same
  structure for comparability.

## Checking correctness

`run.sh` does not diff outputs. To verify all five languages agree:

```sh
for b in fib sieve mandelbrot nqueens matmul hanoi; do
    go run ./$b/$b.go > /tmp/$b.ref
    for cmd in "python3 $b/$b.py" "lua $b/$b.lua" "node $b/$b.js" "goblin run $b/$b.goblin"; do
        $cmd | diff -q /tmp/$b.ref - >/dev/null || echo "MISMATCH: $b <- $cmd"
    done
done
```
