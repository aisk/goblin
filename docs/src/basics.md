# Values, variables, and expressions

Goblin is dynamically typed: variables do not declare a type, and values carry
their types at runtime. Use `var` to bind a name to a value.

```goblin
var project = "Goblin Book"
var published = false
```

Assign to the name to update it later.

```goblin
var score = 10
score = score + 5
print(score) # 15
```

`print` separates multiple arguments with spaces and ends the line. A comment
starts with `#` and runs to the end of its line.

## Operators

The ordinary arithmetic operators are `+`, `-`, `*`, and `/`. They follow the
usual precedence rules; use parentheses to make grouping explicit.

```goblin
print(1 + 2 * 3)       # 7
print((1 + 2) * 3)     # 9
print("go" + "blin")  # goblin
print("ha" * 3)        # hahaha
```

Comparison operators are `==`, `!=`, `<`, `<=`, `>`, and `>=`; each produces a
boolean. Logical operators are `!`, `&&`, and `||`.

```goblin
var allowed = age >= 18 && !banned
```

Conditions use truthiness. `false`, `nil`, numeric zero, and empty strings or
collections are false. Logical expressions always produce booleans. Continue
with [Built-in types](./built-in-types.md) for the values that Goblin provides.
