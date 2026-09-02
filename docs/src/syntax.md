# Syntax and call rules

Goblin uses braces for blocks and does not use semicolons. A newline ends a
statement, so each statement sits on its own line. Comments begin with `#`
and continue to the end of the line.

## Literals and collection syntax

Integer literals such as `42` and float literals such as `3.14` are decimal
only. Strings use double quotes. They support `\n`, `\t`, `\r`, `\"`, and `\\`.
Any other escape sequence is a syntax error.

~~~goblin
var names = ["Ada", "Linus"]
var user = {"name": "Ada", "active": true}
~~~

List, dictionary, call, parameter, and field lists do not accept a trailing
comma. Dictionary keys and values are expressions, but keys should be stable
values such as strings, integers, or booleans.

## Statements and line breaks

Any expression can stand alone as a statement, but only a call, index, or
member access may do so in a program: `user.save()`, `"a,b".split(",")`, and
`func() { ... }()` are valid statements, while `a + b` or a lone `- 2` is
rejected with `expression value is not used`, since a value that is computed
and dropped is almost always a mistake. The REPL is the exception: there a bare
expression such as `1 + 2` is evaluated and displayed.

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
    "b.txt": "second"
}
print(files["a.txt"],
      files["b.txt"])
~~~

The same rule means `else` must follow the closing brace of its `if` on the
same line, and two statements cannot share a line.

## Names

Identifiers may not be Go keywords: names such as `map`, `range`, `select`,
`struct`, `go`, or `defer` are reserved and rejected at check time, because
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
`value.attributes()` in the REPL to discover a value's available methods.
