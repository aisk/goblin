# Control flow

Goblin uses braces for blocks and does not need semicolons.

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
