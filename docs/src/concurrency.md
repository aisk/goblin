# Concurrency

`spawn(function, args...)` starts a Goblin function in a new goroutine. It
returns nil immediately and accepts only positional arguments. The spawned
function's return value and unhandled error are discarded, so a channel is the
normal way to communicate a result back to the caller.

~~~goblin
func square(value, result) {
    result.send(value * value)
}

var result = Chan()
spawn(square, 6, result)
print(result.recv()) # 36
result.close()
~~~

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

An error raised inside a spawned function is not delivered to the caller.
Catch it in the worker and send a result record instead when the caller needs
to handle failures.

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

Goblin has no select operation, cancellation primitive, timeout receive, or
automatic joining of spawned functions. Design each concurrent operation so
every blocking send has a receiver, every expected result is received, and the
owner can decide when it is safe to close its channel.
