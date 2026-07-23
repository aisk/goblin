# random

The random module adapts Go's `math/rand` package for Goblin programs. It is
intended for simulations, games, and tests, not passwords, tokens, or keys.
Module-level functions use a private, automatically seeded generator.

~~~goblin
import "random"

print(random.int())
print(random.int(10))
print(random.float())
~~~

## API

| Function | Go equivalent |
| --- | --- |
| `int(n=unit)` | `Rand.Int63` or `Rand.Int63n` |
| `float()` | `Rand.Float64` |
| `perm(n)` | `Rand.Perm` |
| `shuffle(list)` | `Rand.Shuffle` |
| `normal()` | `Rand.NormFloat64` |
| `exponential()` | `Rand.ExpFloat64` |
| `Generator(seed=unit)` | `rand.New(rand.NewSource(seed))` |

`int()` returns a non-negative `Int`. Supplying `n` returns a value in `[0, n)`
and requires a positive bound. The optional argument combines Go's `Int63` and
`Int63n` methods without adding a separate Goblin name.

`float()` returns a value in `[0.0, 1.0)`. `normal()` draws from the standard
normal distribution with mean zero and standard deviation one.
`exponential()` draws from the exponential distribution with rate one.

`perm(n)` returns a new list containing `[0, n)` in random order. `shuffle`
adapts Go's callback-based method to Goblin by rearranging a list in place and
returning `unit`.

## Independent generators

Use `Generator(seed=unit)` for isolated or reproducible random state. An
explicit integer seed produces the same sequence; omitting it chooses a seed
automatically. The read-only `seed` attribute records the chosen value.

~~~goblin
var first = random.Generator(seed=42)
var second = random.Generator(seed=42)
print(first.int(1000) == second.int(1000))
~~~

A Generator is safe to share between spawned Goblin functions. Calls do not
race internally, though concurrent scheduling determines which caller receives
each successive value.
