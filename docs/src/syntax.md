# Syntax and call rules

Goblin uses braces for blocks and does not use semicolons. Whitespace separates
tokens, while newlines are mainly for readability. Comments begin with `#` and
continue to the end of the line.

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

## Expression statements

A call, index, or member-access chain can stand alone as a statement when it
starts from a name, a string literal, a dictionary literal, or a function
literal — `user.save()`, `"a,b".split(",")`, and `func() { ... }()` are all
valid statements. A statement cannot start with `[` or `(`: because newlines
are not statement terminators, such a line would be indistinguishable from an
index or call continuation of the previous statement. Bind the value with
`var` first instead.

## Names

Identifiers may not be Go keywords: names such as `map`, `range`, `select`,
`struct`, `go`, or `defer` are reserved and rejected at check time, because
compiled programs emit them as Go identifiers. A handful of names the
generated code itself relies on — `object`, `extension`, `builtin`, `fmt`,
`_registry`, `Execute`, and `main` — are reserved for the same reason.
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
