# Modules and errors

## Importing modules

Use `import` for standard modules and local modules. The imported module name
comes from the last part of its path. Imports belong at module scope.

```goblin
import "json"
import "./modules/greeter"

var value = json.unmarshal("{\"ok\": true}")
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

Local import paths start with `./` or `../` and are resolved relative to the
file that contains the import. Add neither the `.goblin` suffix nor a module
name alias: `import "./modules/greeter"` makes `greeter` available.

Only values named by `export` are visible to another module. Imports, function
definitions, and type definitions are resolved before a module's other
top-level statements run, so exported functions may refer to definitions later
in the same module.

## Standard module example

The `json` module turns Goblin values into JSON text and back:

```goblin
import "json"

var encoded = json.marshal({"name": "Ada", "active": true})
var decoded = json.unmarshal(encoded)
print(decoded["name"])
```

`json.unmarshal` raises `ParseError` for invalid JSON. File-system and network
operations can likewise raise errors, so place fallible work inside a `try`
block when it can be handled locally.

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

The name after `catch` is local to the catch block. If no error is raised, the
catch block is skipped. If an error is raised and no enclosing `catch` handles
it, program execution stops and reports a traceback.

Use a sentinel error when callers need to distinguish one failure from another:

```goblin
var not_found = Error("not found")

func load_user(id) {
    if id == 0 {
        raise not_found.wrap("loading user")
    }
    return {"id": id}
}

try {
    load_user(0)
} catch err {
    if err.is(not_found) {
        print("choose another user")
    }
}
```

`Error("message")` creates a plain error. `wrap("context")` adds context,
`unwrap()` returns the direct cause, and `is()` checks an error chain. Runtime
errors can also be checked by kind, including `IndexError`, `ValueError`, and
`ZeroDivisionError`. Built-in error kinds are hierarchical: for example,
`ZeroDivisionError` is also an `ArithmeticError`, and `IndexError` is also a
`LookupError`.

Wrap errors at the boundary that adds useful context. Avoid catching an error
only to discard it; either recover with a meaningful alternative or re-raise it
with `raise err` so an outer layer can decide what to do.
