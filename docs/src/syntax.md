# Syntax and call rules

Goblin uses braces for blocks and does not use semicolons. A newline ends a
statement, so each statement sits on its own line. Comments begin with `#`
and continue to the end of the line.

## Literals and collection syntax

Integer literals are decimal (`42`), hexadecimal (`0x1F`), octal (`0o644`)
or binary (`0b1010`); the prefix is lowercase, hexadecimal digits may be either
case. A decimal integer cannot start with `0`: `0644` is an error, so it can
never be mistaken for an octal number. Float literals have a decimal point
(`3.14`), an exponent (`1e9`), or both (`1.5e-3`). An `_` may separate two
digits in any number literal (`1_000_000`, `0xFF_FF`); anywhere else it is an
error. An integer literal outside the 64-bit range is an error too.
Strings use double quotes, and `{expression}` inside one inserts the
expression's value; backtick strings are raw, with no escapes and no
interpolation, and may span lines. Double-quoted strings support `\n`, `\t`,
`\r`, `\"`, `\\`, `\{`, `\}`, `\0`, `\xHH` and `\u{H...}`; any other escape
sequence is a syntax error. See [Strings](./strings.md) for the details.

~~~goblin
var names = ["Ada", "Linus"]
var user = {"name": "Ada", "active": true}
~~~

List, dictionary, call, parameter, and field lists accept a trailing comma
after the last item, so every line of a multi-line list can end the same way.
A comma with no item before it, as in `[,]` or `f(,)`, is a syntax error.
Dictionary keys and values are expressions, but keys should be stable
values such as strings, integers, or booleans.

## Statements and line breaks

Any expression can stand alone as a statement, but only a call, index, or
member access may do so in a program: `user.save()`, `"a,b".split(",")`, and
`func() { ... }()` are valid statements, while `a + b` or a lone `- 2` is
rejected with `expression value is not used`, since a value that is computed
and dropped is almost always a mistake. The REPL is the exception: there a bare
expression such as `1 + 2` is evaluated and displayed.

A slice is not among them: like `a + b`, `values[1:3]` on its own only
computes a value.

Because a newline ends a statement, a line can only continue the previous one
in places where the expression is visibly unfinished: after a binary operator,
a comma, a dictionary colon, or inside an open `(`, `[`, or `{` of a literal.
Ending a line with a complete expression and starting the next with an
operator does not continue it. `var a = 10` followed by a line `- 2` is two
statements, and the second is the unused-value error above rather than a
silent `10 - 2`.

~~~goblin
var total = price * quantity +
    shipping
var discounted = (
    total - coupon
)
var files = {
    "a.txt": "first",
    "b.txt": "second",
}
print(files["a.txt"],
      files["b.txt"])
~~~

Inside the brackets of a slice, a newline may follow `[` or precede `]`, but
not follow the colon.

The same rule means `else` must follow the closing brace of its `if` on the
same line, and two statements cannot share a line.

## Names

Identifiers may not be Go keywords: names such as `map`, `range`, `select`,
`struct`, `go`, or `switch` are reserved and rejected at check time, because
compiled programs emit them as Go identifiers. A handful of names the
generated code itself relies on — `object`, `extension`, `builtin`, `fmt`,
`_registry`, `Execute`, and `main` — are reserved for the same reason, as is
the whole family of identifiers shaped like `_name_N` (a leading underscore
with a trailing `_<digits>` suffix, e.g. `_err_0`), which the transpiler uses
for its own temporaries.
Built-in function names like `print` or `max` are not reserved — see
[Scope and declarations](./scope.md) for how user declarations shadow them.

## Functions and calls

Function parameters are required unless they declare a default value with `=`
or are captured by `*args` or `**kwargs`.

~~~goblin
func report(name, limit = 10, *values, **options) {
    print(name, limit, values, options)
}

report("scores", 1, 2, visible=true)
~~~

Calls can use positional arguments, named arguments, `*` list expansion, and
`**` dictionary expansion. Positional arguments must come before named ones.
Whether a particular built-in or method accepts names is API-specific: many
small methods are positional-only. Use the documented parameter names or
`attributes(value)` in the REPL to discover a value's available methods.
