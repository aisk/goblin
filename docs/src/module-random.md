# random

The random module provides pseudo-random values for simulations, games, tests,
and other non-security-sensitive work. Module-level functions use a private,
automatically seeded generator.

~~~goblin
import "random"

print(random.int(10))
print(random.int(min=-5, max=6))
print(random.float())
print(random.choice(["red", "green", "blue"]))
~~~

Do not use this module for passwords, tokens, keys, or other security-sensitive
material.

## Integers and floats

`int()` returns a non-negative integer. `int(max)` returns an integer in the
half-open interval `[0, max)`. Use keyword arguments to specify both bounds:

~~~goblin
var offset = random.int(min=-10, max=11)
~~~

The range must satisfy `min < max`; invalid ranges raise `ValueError`. The
implementation supports ranges crossing zero and ranges close to the complete
`Int` domain without overflow or modulo bias.

`float()` returns a value in `[0.0, 1.0)`.

## Choosing and arranging values

`choice(list)` returns one element from a non-empty list. It returns the
original element rather than a copy. An empty list raises `ValueError`.

`shuffle(list)` rearranges a list in place and returns `unit`:

~~~goblin
var cards = ["ace", "king", "queen"]
random.shuffle(cards)
~~~

`perm(n)` returns a new list containing every integer in `[0, n)` exactly once
in random order. It does not modify another value. `n` must be non-negative.

~~~goblin
var indexes = random.perm(5)
~~~

## Independent generators

Use `Generator(seed=unit)` when random state must be isolated or reproducible.
It provides the same `int`, `float`, `choice`, `shuffle`, and `perm` operations
as the module.

~~~goblin
var first = random.Generator(seed=42)
var second = random.Generator(seed=42)

print(first.int(1000) == second.int(1000)) # true
~~~

Omitting the seed chooses one automatically. The chosen value is available as
the read-only `seed` attribute so a run can be reproduced later:

~~~goblin
var rng = random.Generator()
print("seed:", rng.seed)
~~~

A Generator is safe to share between spawned Goblin functions. Calls do not
race internally, but concurrent scheduling determines which caller receives
each successive value, so concurrent sequences are not reproducible.

The module-level functions use an internal Generator rather than Go's global
random-number state, so other Go extensions cannot silently alter Goblin's
sequence.
