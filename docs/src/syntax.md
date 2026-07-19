# Syntax and call rules

Goblin uses braces for blocks and does not use semicolons. Whitespace separates
tokens, while newlines are mainly for readability. Comments begin with `#` and
continue to the end of the line.

## Literals and collection syntax

Integer literals such as `42` and float literals such as `3.14` are decimal
only. Strings use double quotes. They support `\n`, `\t`, `\r`, `\"`, and `\\`.
An unrecognised escape drops its backslash and keeps the following character.

~~~goblin
var names = ["Ada", "Linus"]
var user = {"name": "Ada", "active": true}
~~~

List, dictionary, call, parameter, and field lists do not accept a trailing
comma. Dictionary keys and values are expressions, but keys should be stable
values such as strings, integers, or booleans.

## Functions and calls

Function parameters are required unless captured by `*args` or `**kwargs`.
Goblin does not have default function parameter values.

~~~goblin
func report(name, *values, **options) {
    print(name, values, options)
}

report("scores", 1, 2, visible=true)
~~~

Calls can use positional arguments, named arguments, `*` list expansion, and
`**` dictionary expansion. Positional arguments must come before named ones.
Whether a particular built-in or method accepts names is API-specific: many
small methods are positional-only. Use the documented parameter names or
`value.attributes()` in the REPL to discover a value's available methods.
