# Control flow

Goblin uses braces for blocks and does not need semicolons.

An indented style makes nested blocks easy to scan, but indentation is not part
of the grammar: braces determine the block.

## Conditions

Use `if`, `else if`, and `else` to select a branch:

```goblin
if score >= 90 {
    print("excellent")
} else if score >= 60 {
    print("passed")
} else {
    print("try again")
}
```

Every condition is evaluated with the truthiness rules described in [Built-in
types](./built-in-types.md). This lets you test optional values and collections
directly:

```goblin
var users = []
if users {
    print("have users")
} else {
    print("no users")
}
```

## `while` loops

A `while` loop repeats while its condition is true. `break` leaves the nearest
loop and `continue` skips to its next iteration.

```goblin
var n = 0
while n < 10 {
    n = n + 1
    if n == 3 {
        continue
    }
    if n == 6 {
        break
    }
    print(n)
}
```

Be sure that a `while` condition eventually changes or that the loop reaches a
`break`; Goblin does not impose a loop limit. In nested loops, `break` and
`continue` apply only to the innermost loop.

## `for ... in` loops

`for` iterates over lists, strings, dictionaries, and other iterable values.
Iterating over a dictionary produces its keys; do not depend on their order.

```goblin
for name in ["Ada", "Linus"] {
    print(name)
}

for i in range(0, 3) {
    print(i) # 0, 1, 2
}
```

`range(start, end)` creates integers from `start` up to, but excluding, `end`.
It also accepts named arguments, such as `range(start=0, end=3)`.

Strings iterate by character, lists by element, and dictionaries by key:

```goblin
var scores = {"Ada": 10, "Linus": 9}
for name in scores {
    print(name, scores[name])
}

for character in "go" {
    print(character)
}
```

Dictionary order is unspecified. If output order matters, do not build it by
iterating a dictionary directly.
