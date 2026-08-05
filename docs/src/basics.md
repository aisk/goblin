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

`print` separates multiple arguments with spaces and ends the line; it writes
to stdout. Use `eprint` for the same formatting on stderr. A comment starts
with `#` and runs to the end of its line.

## Operators

The ordinary arithmetic operators are `+`, `-`, `*`, `/`, and `%`. The `%`
operator returns the remainder and has the same precedence as multiplication
and division. Use parentheses to make grouping explicit.

```goblin
print(1 + 2 * 3)       # 7
print((1 + 2) * 3)     # 9
print("go" + "blin")  # goblin
print("ha" * 3)        # hahaha
print(10 % 3)          # 1
```

Comparison operators are `==`, `!=`, `<`, `<=`, `>`, and `>=`; each produces a
boolean. Equality is total across types: values of unrelated types are simply
unequal, so `x == nil` is always a safe test, and lists and dictionaries
compare element by element. Ordering comparisons (`<`, `<=`, `>`, `>=`) raise
TypeError when the operands cannot be ordered. Comparisons do not chain:
`a < b < c` is a syntax error rather than the surprising `(a < b) < c`, so
write `a < b && b < c`, or parenthesize explicitly when you really mean to
compare a boolean result. Logical operators are `!`, `&&`, and `||`.
`&&` and `||` short-circuit, so their right-hand side is evaluated only when
needed.

```goblin
var allowed = age >= 18 && !banned
```

`&&` binds tighter than `||`, as in most languages: `ready || retry &&
connected` evaluates as `ready || (retry && connected)`, and
`true || false && false` produces `true`.

Conditions use truthiness. `false`, `nil`, numeric zero, and empty strings or
collections are false. Logical expressions always produce booleans. Continue
with [Built-in types](./built-in-types.md) for the values that Goblin provides.
