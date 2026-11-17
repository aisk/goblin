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

wrap("context") adds a message while preserving the original error. The
`cause` property is the next error down the chain, or nil at the bottom, and
is() tests the complete chain, the error itself included.

~~~goblin
var base = Error("connection failed")
var err = base.wrap("loading profile")
print(err.message)       # loading profile: connection failed
print(err.cause.message) # connection failed
print(err.cause == base) # true
print(err.is(base))      # true
print(base.cause)        # nil
~~~

The chain continues through error kinds: a kind's `cause` is its parent kind,
so `IndexError.cause == LookupError`, and the base `Error` kind, at the top,
has a nil cause. A failure reported by a library module carries its kind one
level down: when `fs.read` fails, `e.message` describes what failed,
`e.cause` is the operating system's own error (`open nope.txt: no such file
or directory`), and `e.cause.is(NotExistError)` is true.

The error a `catch` binds is the error that was raised, however many calls it
passed through on the way: `e == not_found` and `e.cause` give the same answer
for a raise at module level and one three functions deep. Errors compare and
hash by identity, so a sentinel can key a dictionary and a caught error finds
its entry.

## Error kinds

The runtime uses named error kinds, which can be raised directly or wrapped.

| Kind | Typical cause |
| --- | --- |
| TypeError | An operation receives an incompatible value |
| ValueError | A valid type has an invalid value |
| IndexError / KeyError | A missing list index or dictionary key |
| ZeroDivisionError | Division or modulo by zero |
| OverflowError | An integer result that does not fit in 64 bits |
| AttributeError / NameError | A missing member or identifier |
| ImportError | An unavailable module |
| ParseError | Invalid JSON or other parsed input |
| IOError | Generic file, network, or operating-system failure |
| NotExistError / ExistError / PermissionError | A missing, existing, or inaccessible filesystem path |
| TimeoutError / NetworkError | A timed-out or other network operation |
| NotImplementedError | An operation the runtime does not implement |
| RecursionError | Recursion, or comparing or printing nested values, too deep for the stack |

Kinds are hierarchical. IndexError and KeyError are LookupErrors;
ZeroDivisionError and OverflowError are ArithmeticErrors; ParseError is a ValueError. Therefore
err.is(LookupError) can handle more than one specific lookup failure.

## Cleaning up

There is no `finally`. To release something whether or not an error occurs,
`defer` the release right after acquiring it: a deferred call runs when the
function ends, including when an error is on its way out of it.

~~~goblin
func load(path) {
    var file = fs.open(path)
    defer file.close()
    return parse(file.read())   # file.close() runs even if parse raises
}
~~~

See [Deferred calls](./functions.md#deferred-calls) for the details.

Wrap errors where you can add useful context. Do not catch an error merely to
discard it: either recover with a meaningful alternative, or use raise err to
let an outer layer handle it.
