# Concurrency

There are two ways to start a function in a new goroutine. `Goblin(function,
args...)` starts it immediately and returns a handle; `wait()` on the handle
joins the function and delivers its result. `spawn(function, args...)` is
fire-and-forget: it returns nil immediately, and the function's return value
and unhandled error are discarded. Use a Goblin handle when the caller cares
about the outcome, and spawn when it does not.

~~~goblin
func square(value) {
    return value * value
}

var worker = Goblin(square, 6)
print(worker.wait()) # 36
~~~

## Goblin handles

Construction starts the function right away; there is no separate start step.
Arguments after the function are forwarded to it.

| Operation | Behavior |
| --- | --- |
| `Goblin(function, args...)` | Start the function in a new goroutine and return a handle |
| `handle.wait()` | Block until the function finishes, then return its result |
| `handle.wait(timeout = seconds)` | Same, but raise TimeoutError if the function is still running when the timeout expires |
| `handle.done()` | Report whether the function has finished, without blocking |

The outcome is computed once and cached, so `wait()` can be called any number
of times — and from several places at once — with the same answer. An error
raised inside the function is not printed anywhere; it is stored and re-raised
by every `wait()` call, where the normal `try`/`catch` machinery applies:

~~~goblin
func fail() {
    raise ValueError.wrap("bad input")
}

var failing = Goblin(fail)
try {
    failing.wait()
} catch err {
    print(err.is(ValueError)) # true
}
~~~

A handle that is never waited on is abandoned: when the program ends, running
goblins are stopped mid-flight, and a stored error nobody asked for is
discarded silently. If a function's failure matters, `wait()` for it; if the
result never matters, `spawn` is the honest way to say so (an uncaught error
in a spawned function is at least reported on stderr).

## Sharing data with a goblin

Pass data into a goblin through its arguments, and out through its return
value or a channel. Two goblins — or a goblin and the top-level program —
reading and writing the same variable concurrently is a data race: no ordering
is defined, and the compiled backend inherits Go's undefined behavior for
races. Reading surrounding names that nothing writes concurrently (module
imports, top-level functions, builtins) is safe.

## Channels

`Chan()` and `Chan(0)` create an unbuffered channel. A send waits until some
goroutine receives it, and a receive waits until a value is sent. `Chan(size)`
creates a buffer that can hold up to `size` values before sends block.

| Operation | Behavior |
| --- | --- |
| `channel.send(value)` | Blocks until a receiver is ready or buffer space exists |
| `channel.recv()` | Blocks until a value is available |
| `channel.close()` | Prevents future sends; buffered values can still be received |
| `Chan(size)` | Requires a non-negative integer; omitting size means zero |

Sending on a closed channel, closing a channel twice, or receiving after a
channel is closed and drained raises ValueError. Channels are not iterable and
there is no special end-of-stream value, so a receiver must know how many
values to expect or use a separate completion signal.

## Buffering a known number of results

Use a buffered channel when several workers can finish before the caller starts
receiving. The buffer capacity here matches the number of results.

~~~goblin
func square(value, result) {
    result.send(value * value)
}

var results = Chan(3)
for value in [2, 3, 4] {
    spawn(square, value, results)
}

var total = 0
for ignored in range(0, 3) {
    total = total + results.recv()
}
results.close()
print(total) # 29
~~~

An unbuffered `results` channel also works in this example because the caller
begins receiving after it starts the workers. Buffering changes *when* sends
block; it does not guarantee a result order. Do not rely on spawned work
finishing in the order it was started.

## Returning errors explicitly

`wait()` re-raises a goblin's error at the call site, which covers the common
case of one job whose failure the caller handles. When workers stream results
through a channel instead, an error raised inside a spawned function is not
delivered to the caller. Catch it in the worker and send a result record when
the caller needs to handle failures.

~~~goblin
func load_number(text, result) {
    try {
        result.send({"value": Int(text), "error": nil})
    } catch err {
        result.send({"value": nil, "error": err})
    }
}

var result = Chan()
spawn(load_number, "not-a-number", result)
var outcome = result.recv()
if outcome["error"] {
    print(outcome["error"].message)
}
result.close()
~~~

Use a dictionary only as a small result record like this. For a repeated or
larger protocol, define a custom type so the fields and methods are explicit.

## Ownership and deadlocks

The code that knows no more values will be sent should close the channel. Do
not close a channel while a spawned sender may still use it. A common deadlock
is sending to an unbuffered channel in the same goroutine before starting a
receiver:

~~~goblin
var messages = Chan()
# messages.send("hello") would block here: no receiver can run yet.
spawn(func() { messages.send("hello") })
print(messages.recv())
messages.close()
~~~

Goblin has no select operation, no cancellation primitive, and no timeout on
channel operations; `wait(timeout = seconds)` on a Goblin handle is the only
time-bounded wait, and functions started with `spawn` cannot be joined at all.
Design each concurrent operation so every blocking send has a receiver, every
expected result is received, and the owner can decide when it is safe to close
its channel.
