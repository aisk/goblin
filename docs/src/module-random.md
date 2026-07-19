# random

The random module provides convenient pseudo-random values for simulations,
examples, and non-security-sensitive choices.

~~~goblin
import "random"

print(random.intn(10))
print(random.float())
print(random.choice(["red", "green", "blue"]))
~~~

int() returns a non-negative integer. intn(limit) returns an integer from zero
up to, but excluding, limit; limit must be positive. float() returns a float
from zero up to, but excluding, one. choice(list) returns one element from a
non-empty list.

Use random values for games or sampling. Do not use this module for passwords,
tokens, or other security-sensitive material. intn() and choice() raise
ValueError for invalid limits or empty lists.

## Selecting and sampling

choice() returns the original list element, so it works with any list element
type, not just strings. Combine intn() with list indexing when the program also
needs to know the selected position.

~~~goblin
var servers = ["a.example", "b.example", "c.example"]
var index = random.intn(servers.size())
print(index, servers[index])
~~~

float() is useful for probabilities. Compare it with a threshold to choose a
weighted branch.

~~~goblin
if random.float() < 0.1 {
    print("rare event")
} else {
    print("ordinary event")
}
~~~

The generator is seeded automatically. There is no seed-setting API, so tests
should assert ranges or properties rather than a fixed random sequence.
