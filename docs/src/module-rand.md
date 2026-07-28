# rand

The `rand` module provides pseudo-random values backed by Go's `math/rand`
package. It is suitable for simulations, games, and reproducible tests, but not
for passwords, tokens, keys, or other security-sensitive work.

~~~goblin
import "rand"

print(rand.int(10))
print(rand.float())
~~~

Module-level functions use a private, automatically seeded generator.

## Module API

| Function | Description |
| --- | --- |
| `Rand(seed)` | Create an independent generator with an integer seed. |
| `int(max=nil)` | Return a non-negative integer, optionally below `max`. |
| `float()` | Return a Float in `[0.0, 1.0)`. |
| `perm(n)` | Return a random permutation of the integers `[0, n)`. |
| `shuffle(list)` | Rearrange a list in place and return `unit`. |
| `norm_float()` | Draw from the standard normal distribution. |
| `exp_float()` | Draw from the exponential distribution with rate 1. |

`int()` corresponds to Go's `Int63`. Passing `max` selects the bounded
`Int63n` behavior; merging those two Go functions is possible because Goblin
supports optional arguments. `max` must be positive.

`shuffle` accepts a Goblin list instead of Go's length-and-callback pair. The
callback exists only to let Go swap values of arbitrary static types; exposing
it would leak that implementation constraint into Goblin.

## Independent generators

`Rand(seed)` wraps Go's `rand.Rand`. Two instances created with the same seed
produce the same sequence:

~~~goblin
var first = rand.Rand(42)
var second = rand.Rand(42)

print(first.int(1000) == second.int(1000)) # true
~~~

A `Rand` provides the same `int`, `float`, `perm`, `shuffle`, `norm_float`, and
`exp_float` methods as the module. It is safe to share between spawned Goblin
functions. Concurrent scheduling still determines which caller receives each
successive value, so concurrent sequences are not reproducible.

Unlike the old `random.Generator`, construction always requires a seed. This
keeps independent generator creation conceptually aligned with Go's
`rand.New(rand.NewSource(seed))`; use the module functions when reproducibility
is not required.

## Deliberately omitted Go API

- `Source`, `Source64`, and `New` are replaced by `Rand(seed)`. Go's source
  interfaces are extension points for Go implementations and should not leak
  into the Goblin value API.
- `Read` is omitted because its Go signature fills a caller-owned byte slice.
  A future byte-stream API should follow Goblin's common reader protocol rather
  than expose Go buffer mutation.
- Integer-width variants (`Int31`, `Int63`, `Uint32`, and `Uint64`) are not
  separate functions because Goblin has one signed 64-bit `Int` type.
- `Seed` is omitted. Reproducible state belongs to an explicit `Rand` rather
  than mutable module-global state.
- `Zipf` is omitted from the initial useful subset because it requires a
  specialized stateful distribution object.

The former `random` module and its Python-inspired `choice`, `sample`, bounded
`float`, parameterized `normal`, and parameterized `exponential` functions were
removed. They do not correspond to the Go package being wrapped.
