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
