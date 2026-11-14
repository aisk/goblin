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

## Destructuring

A `var` or an assignment can take several targets at once. The value on the
right is iterated, and each element goes to the target in the same position:

```goblin
var width, height = [640, 480]
print(width, height) # 640 480

width, height = [height, width]
print(width, height) # 480 640
```

The right side is evaluated in full before anything is stored, which is why
the swap above needs no temporary. An assignment target may be anything a
plain assignment accepts, so `point.x, items[0] = pair` works too. Any
iterable can be destructured, though a list is the usual choice. The number
of elements must equal the number of targets; otherwise the statement raises
ValueError and stores nothing.

A function returns several results the same way, as one list the caller takes
apart:

```goblin
func divmod(a, b) {
    return [a / b, a % b]
}

var quotient, remainder = divmod(17, 5)
```

A `for` loop destructures each element with the same syntax; see
[Control flow](./control-flow.md).

## Printing and comments

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
needed, and they produce the operand that decided the result rather than a
boolean: `a || b` is `a` when `a` is truthy and `b` otherwise, `a && b` is `a`
when `a` is falsy and `b` otherwise. That makes `||` the way to write "use
this, or a default". Wrap the expression in `Bool()` when you need a plain
true/false. `!` always produces a boolean.

```goblin
var allowed = age >= 18 && !banned
var nickname = supplied || "anonymous"
```

`&&` binds tighter than `||`, as in most languages: `ready || retry &&
connected` evaluates as `ready || (retry && connected)`, and
`true || false && false` produces `true`.

Conditions use truthiness. `false`, `nil`, numeric zero, and empty strings or
collections are false. Continue
with [Built-in types](./built-in-types.md) for the values that Goblin provides.
