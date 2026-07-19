# Modules and errors

## Importing modules

Use `import` for standard modules and local modules. The imported module name
comes from the last part of its path.

```goblin
import "json"
import "./modules/greeter"

var value = json.unmarshal("{\\"ok\\": true}")
greeter.greet("world")
```

A local module explicitly chooses what it exports:

```goblin
# modules/greeter.goblin
var greeting = "Hello"
func greet(name) {
    print(greeting, name)
}
export greeting
export greet
```

Common standard modules include `json`, `os`, `fs`, `path`, `random`, and
`time`. Import a module before using it.

## Error handling

Errors are values. Raise one with `raise` and handle it with `try` / `catch`:

```goblin
func divide(a, b) {
    if b == 0 {
        raise ZeroDivisionError.wrap("divide")
    }
    return a / b
}

try {
    print(divide(10, 0))
} catch err {
    print(err.message)
    print(err.is(ZeroDivisionError))
}
```

`Error("message")` creates a plain error. `wrap("context")` adds context,
`unwrap()` returns the direct cause, and `is()` checks an error chain. Runtime
errors can also be checked by kind, including `IndexError`, `ValueError`, and
`ZeroDivisionError`.
