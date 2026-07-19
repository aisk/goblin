# Errors

Errors are values. Use raise to stop normal execution with an error and try /
catch to recover at a boundary that can make a useful decision.

~~~goblin
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
~~~

The name after catch is local to its catch block. When no error is raised, the
catch block is skipped. An unhandled error stops the program and prints a
traceback.

## Creating and matching errors

Error("message") creates a plain error. Use a sentinel error when callers need
to distinguish a particular failure from unrelated errors.

~~~goblin
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
~~~

wrap("context") adds a message while preserving the original error. unwrap()
returns the direct cause, and is() tests the complete error chain.

~~~goblin
var base = Error("connection failed")
var err = base.wrap("loading profile")
print(err.message)
print(err.unwrap().message)
print(err.is(base))
~~~

## Error kinds

The runtime uses named error kinds, which can be raised directly or wrapped.

| Kind | Typical cause |
| --- | --- |
| TypeError | An operation receives an incompatible value |
| ValueError | A valid type has an invalid value |
| IndexError / KeyError | A missing list index or dictionary key |
| ZeroDivisionError | Division by zero |
| AttributeError / NameError | A missing member or identifier |
| ImportError | An unavailable module |
| ParseError | Invalid JSON or other parsed input |
| IOError | Generic file, network, or operating-system failure |
| NotExistError / ExistError / PermissionError | A missing, existing, or inaccessible filesystem path |
| TimeoutError / NetworkError | A timed-out or other network operation |
| NotImplementedError | An operation the runtime does not implement |

Kinds are hierarchical. IndexError and KeyError are LookupErrors;
ZeroDivisionError is an ArithmeticError; ParseError is a ValueError. Therefore
err.is(LookupError) can handle more than one specific lookup failure.

Wrap errors where you can add useful context. Do not catch an error merely to
discard it: either recover with a meaningful alternative, or use raise err to
let an outer layer handle it.
