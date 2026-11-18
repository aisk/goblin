# Errors

Errors are values. Use raise to stop normal execution with an error and try /
catch to recover at a boundary that can make a useful decision.

~~~goblin
func divide(a, b) {
    if b == 0 {
        raise ZeroDivisionError("cannot divide {a} by zero")
    }
    return a / b
}

try {
    print(divide(10, 0))
} catch err: ZeroDivisionError {
    print(err.message)   # cannot divide 10 by zero
}
~~~

The name after catch is local to its catch block. When no error is raised, the
catch block is skipped. An unhandled error stops the program and prints a
traceback.

## Creating and matching errors

There are two ways to derive an error, and they have different jobs:

- **Call a kind** to report a new failure of that kind. `ValueError("invalid
  port: {s}")` is an error whose message is exactly that and whose `cause` is
  `ValueError`, so it matches `ValueError` (and its parent kinds).
- **Wrap an error** that already happened to add context.
  `err.wrap("loading profile")` is an error whose message is
  `"loading profile: "` followed by the original message, and whose `cause` is
  the original error.

`Error` is the base kind, the one every other kind descends from, so
`Error("message")` creates an error of the base kind and nothing more specific:
its `cause` is `Error`. Any error can serve as a kind: a kind is simply an error
you call or test for. Use one when callers need to tell a particular failure
apart from unrelated errors.

~~~goblin
var ConfigError = ValueError("ConfigError")   # a kind of your own, a ValueError
var not_found = Error("not found")            # a sentinel, raised as it is

func load_user(id) {
    if id == 0 {
        raise not_found
    }
    return {"id": id}
}

try {
    load_user(0)
} catch err: not_found {
    print("choose another user")
}

var err = ConfigError("missing field 'name'")
print(err.message)           # missing field 'name'
print(err.is(ConfigError))   # true
print(err.is(ValueError))    # true
~~~

Calling a kind with no message keeps the kind's own: `ValueError().message` is
`ValueError`, and `Error().message` is `Error`.

A kind is callable like any constructor, so it can be passed wherever a
function is expected: `["bad port", "bad host"].map(ConfigError)` makes one
ConfigError per message, and `Function(ValueError)` returns the kind itself.

The `cause` property is the next error down the chain, and is() tests the
complete chain, the error itself included.

~~~goblin
var base = Error("connection failed")
var err = base.wrap("loading profile")
print(err.message)       # loading profile: connection failed
print(err.cause.message) # connection failed
print(err.cause == base) # true
print(err.is(base))      # true
print(base.cause)        # Error
print(Error.cause)       # nil
~~~

The chain continues through error kinds: a kind's `cause` is its parent kind,
so `IndexError.cause == LookupError`. Every chain ends at the base kind
`Error`, the only error whose `cause` is nil. A failure reported by a library
module carries its kind one level down: when `fs.read` fails, `e.message`
describes what failed, `e.cause` is the operating system's own error (`open
nope.txt: no such file or directory`), and `e.cause.is(NotExistError)` is
true, as is `e.is(NotExistError)`.

The error a `catch` binds is the error that was raised, however many calls it
passed through on the way: `e == not_found` and `e.cause` give the same answer
for a raise at module level and one three functions deep. Errors compare and
hash by identity, so a sentinel can key a dictionary and a caught error finds
its entry.

## Catching by kind

`catch e: Kind { ... }` handles an error only when `e.is(Kind)` is true;
`catch e { ... }` handles any error. `e.is(Error)` is true for every error,
including the runtime's own failures and wrapped errors, so `catch e: Error`
and `catch e` handle the same errors. A try can have several clauses. They are
tested from top to bottom, and the first that matches runs; the others are
skipped.

~~~goblin
try {
    var text = fs.read(path)
} catch e: NotExistError {
    print("no file yet")
} catch e: PermissionError {
    raise e.wrap("fix the permissions of {path}")
} catch e {
    eprint("unexpected: {e}")
    raise e
}
~~~

- When no clause matches, the error continues outward unchanged: the same
  error value with the same traceback, exactly as if there were no try.
- The kind can be any expression (`NotExistError`, `not_found`, `kinds[i]`).
  It is evaluated only when an error reaches its clause, in the scope around
  the try; the clause's own variable is not visible in it. A kind that is not
  an Error raises `TypeError: catch expects an Error to match, got Int` in
  place of the error being handled, and so does any error raised while
  evaluating the kind.
- A clause without a kind catches everything, so it must be the last one:
  `catch after a catch-all clause can never run` is reported when the program
  is parsed.
- `os.exit()` is not an error: no `catch` clause receives it, with or without a
  kind, `catch e: Error` included.
- Each clause has its own variable and scope; clauses may use different names.
- An error raised inside a clause body is not offered to the later clauses of
  the same try.

## Errors with data

Keyword arguments given when an error is made are data, read back as
attributes:

~~~goblin
var ConfigError = ValueError("ConfigError")

try {
    raise ConfigError("missing field 'name'", field="name", line=3)
} catch e: ConfigError {
    print("{e.message} (line {e.line})")   # missing field 'name' (line 3)
    print(e.field)                          # name
}
~~~

- `e.name` looks in the error's own data and then down its cause chain, the
  first hit winning. So a `wrap` between the raise and the catch does not hide
  the data, and data given to a kind (`var Retryable = Error("retryable",
  retry=true)`) is seen on every error made from it.
- A name that is not there is the usual `AttributeError: Error has no attribute
  'code'`. To probe an error of unknown kind, use
  `attributes(e).contains("code")`; it lists the built-in members and then the
  data names, nearest first.
- Data cannot be changed once the error is made, and the built-in members
  `traceback`, `cause`, `wrap` and `is` cannot be data names (`ValueError:
  Error() cannot use 'cause' as a data name`).
- Data is for code: `print(e)`, `e.message` and the traceback show only the
  message.

## How an error displays

`print(e)`, `String(e)`, interpolation and an uncaught error show the message
alone. Inside a list, a dictionary or a structural `impl Show {}`, a bare
message could not be told apart from a String, from a name, or (when empty)
from nothing, so there an error shows as `Kind("message")`: Kind is the
nearest kind in its cause chain, and the message is quoted as a String
element is. A kind itself, being an error whose message is a capitalized name,
shows as that bare name.

~~~goblin
var ConfigError = ValueError("ConfigError")
var not_found = Error("not found")

print([ValueError("bad input"), "bad input", ValueError("")])
# [ValueError("bad input"), "bad input", ValueError("")]
print([ValueError, ConfigError, ConfigError("missing name")])
# [ValueError, ConfigError, ConfigError("missing name")]
print([not_found, ValueError("bad").wrap("parsing")])
# [Error("not found"), ValueError("parsing: bad")]
~~~

A sentinel such as `not_found` is not a kind by that rule (its message is not
a name), so it shows with the kind it was made from; a wrapped error shows its
full message with the kind of the error it wraps.

## Error kinds

The runtime uses named error kinds. Raise one as it is, or call it to make an
error of that kind with a message of your own. Every kind descends from the
base kind `Error`; the table lists all of them with the kind each one directly
descends from.

| Kind | Parent | Typical cause |
| --- | --- | --- |
| TypeError | Error | An operation receives an incompatible value |
| ValueError | Error | A valid type has an invalid value |
| ParseError | ValueError | Invalid JSON or other parsed input |
| LookupError | Error | Not raised by the runtime directly; it groups the five kinds below it |
| IndexError | LookupError | A list index out of range |
| KeyError | LookupError | A missing dictionary key |
| AttributeError | LookupError | A missing member, such as a misspelled field |
| NameError | LookupError | A missing identifier |
| ImportError | LookupError | An unavailable module |
| ArithmeticError | Error | Not raised by the runtime directly; it groups the two kinds below it |
| ZeroDivisionError | ArithmeticError | Division or modulo by zero |
| OverflowError | ArithmeticError | An integer result that does not fit in 64 bits, or a number too large for its type |
| IOError | Error | Generic file, network, or operating-system failure |
| NotExistError | IOError | A missing filesystem path |
| ExistError | IOError | A filesystem path that already exists |
| PermissionError | IOError | An inaccessible filesystem path |
| TimeoutError | IOError | A `timeout=` that ran out, with or without I/O: a channel's `send` or `receive`, a goblin's `wait`, `wait_any`, or a network request |
| NetworkError | IOError | A failed network operation (a connection refused, reset or unreachable, a broken pipe). A local file failure such as "is a directory" is a plain IOError |
| NotImplementedError | Error | An operation the runtime does not implement |
| RecursionError | Error | Recursion, or comparing or printing nested values, too deep for the stack |
| InternalError | Error | A failure inside the runtime itself, such as a Go panic in a goblin |

A `catch` clause handles its kind and every kind below it. So `catch e:
LookupError` also handles AttributeError, NameError and ImportError, which can
come from a misspelled name rather than a missing entry; catch IndexError or
KeyError when only a missing index or key is expected. Likewise `catch e:
IOError` handles a TimeoutError from a channel or a goblin, not only one from
the file system or the network.

An uncaught error prints its message, not its kind name, so give the errors
you make a message that says what went wrong.

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
discard it: name the kind you can handle, and let everything else pass
through.
