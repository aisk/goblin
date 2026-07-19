# Values, variables, and expressions

Goblin is dynamically typed: variables do not declare a type, and values carry
their types at runtime. The basic values are integers, floats, strings,
booleans, and `nil`.

```goblin
var count = 3
var price = 19.5
var name = "Goblin"
var enabled = true
var missing = nil
```

Declare a variable with `var`. Assign to its name to update it later.

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

Conditions use truthiness. `false`, `nil`, and the numeric value `0` are false.
Logical expressions always produce booleans.

## Strings

Strings use double quotes. Escape a double quote or backslash with a backslash:

```goblin
var message = "say: \\"hello\\""
print(message.size())
```

Strings are values with methods. Useful examples include `upper()`, `lower()`,
`contains()`, `split()`, and `replace()`.

```goblin
var title = "Goblin book"
print(title.upper())
print(title.contains("book"))
print("a,b,c".split(","))
```

Use `Int()`, `Float()`, and `Str()` for explicit conversions:

```goblin
var port = Int("8080")
var label = Str(port)
```
