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

for i in range(3) {
    print(i) # 0, 1, 2
}

for i in range(1, 4) {
    print(i) # 1, 2, 3
}
```

`range(end)` creates the integers from zero up to, but excluding, `end`, and
`range(start, end)` those from `start`. It also accepts named arguments, such
as `range(start=0, end=3)`. Called directly in a loop header, `range` counts
instead of building its list, so `for i in range(1000000000000)` with an early
`break` costs only the iterations that run.

A loop takes one element at a time from the value it walks. For a file that
means one line at a time (see [fs](module-fs.md)): the loop reads only as far
as it gets, and an error such as an invalid line is raised when the loop
reaches it, after the earlier elements have run through the body. A list is
walked over the elements it held when the loop started: assigning an element
the loop has not reached yet is seen, but `push` and `pop` do not change how
many elements it visits. A dictionary loop visits the keys it held when the
loop started.

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

When every element is itself a small sequence, name its parts in the loop
header. Each element is destructured as `var a, b = element` would be, so an
element with a different number of parts raises ValueError:

```goblin
for name, score in scores.items() {
    print(name, score)
}

for x, y in [[0, 1], [2, 3]] {
    print(x + y)
}
```
