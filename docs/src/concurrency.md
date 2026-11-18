# Concurrency

`spawn(function, args...)` starts a function in a new goroutine and returns a
Goblin handle; `wait()` on the handle joins the function and delivers its
result. Channels (`Chan`) carry streams of values between goroutines, and every
blocking operation takes the same `timeout=` keyword. Those three are all
there is: a Goblin handle stands for one running function, a Chan carries
values from one goblin to another, and `timeout=` limits how long any blocking
call waits.

~~~goblin
func square(value) {
    return value * value
}

var worker = spawn(square, 6)
print(worker.wait()) # 36
~~~

## Goblin handles

`spawn` starts the function right away; there is no separate start step.
Arguments after the function, positional and keyword alike, are forwarded to
it. A handle is only ever obtained from `spawn`: `Goblin` is the name of its
type, not a function you can call.

| Operation | Behavior |
| --- | --- |
| `spawn(function, args...)` | Start the function in a new goroutine and return a handle |
| `handle.wait()` | Block until the function finishes, then return its result |
| `handle.wait(timeout = seconds)` | Same, but raise TimeoutError if the function is still running when the timeout expires |
| `handle.done` | Whether the function has finished; reading it never blocks |
| `wait_any(goblins, timeout = nil)` | Block until one of the goblins finishes and return its handle |

The outcome is computed once and cached, so `wait()` can be called any number
of times — and from several places at once — with the same answer. An error
raised inside the function is stored and re-raised by every `wait()` call,
where the normal `try`/`catch` machinery applies:

~~~goblin
func fail() {
    raise ValueError("bad input")
}

var failing = spawn(fail)
try {
    failing.wait()
} catch err {
    print(err.is(ValueError)) # true
}
~~~

A handle you do not need can simply be dropped; `spawn(log_visit, user)` on a
line of its own is fine. An error is still never lost: when the program ends
normally, every goblin that failed without any `wait()` seeing its error is
reported on stderr as `uncaught error in goblin: <error>`, in the order they
failed. The report does not change the exit status. Goblins still running when
the program ends are stopped mid-flight and not reported. The same holds when a
goblin calls `os.exit()`: that goblin runs its deferred calls, every other
goblin is stopped mid-flight, and failed goblins nobody waited for are not
reported. The REPL makes the same report after each line it evaluates.

### Waiting for the first of several

`wait_any(goblins)` takes any iterable of handles and returns the first one to
finish. It does not take the result or raise the error; call `wait()` on the
handle it returns for that. When several have already finished, it returns the
one earliest in the list, so the answer is reproducible; which of several
goblins finishing at the same moment it picks while it waits is not defined.
An empty iterable raises ValueError, an element that is not a Goblin handle
raises TypeError.

~~~goblin
import "time"

func fetch(name, delay) {
    time.sleep(delay)
    return name
}

var jobs = [spawn(fetch, "slow", 1), spawn(fetch, "fast", 0.01)]
var first = wait_any(jobs, timeout = 5)
print(first.wait()) # fast
~~~

There is no cancellation: the goblins that did not finish first keep running
until they return or the program ends.

## Time limits

`wait`, `send`, `receive` and `wait_any` all take an optional `timeout` in
seconds, an Int or a Float. When it expires the operation raises
TimeoutError (`receive() timed out after 2 seconds`). `nil`, the default,
waits forever; a negative timeout raises ValueError and any other type
TypeError. An operation that can complete immediately always does, even with
`timeout = 0`, so `channel.receive(timeout = 0)` means "take a value if one
is ready".

## Sharing data with a goblin

Pass data into a goblin through its arguments, and out through its return
value or a channel. Two goblins — or a goblin and the top-level program —
reading and writing the same variable concurrently is a data race: no ordering
is defined, and the compiled backend inherits Go's undefined behavior for
races. Reading surrounding names that nothing writes concurrently (module
imports, top-level functions, builtins) is safe.

The same rule covers the inside of a shared mutable value: lists and
dictionaries are not synchronized, and Goblin never locks them for you. The
consequence of a race is not limited to stale or torn data — two goblins
writing to the same dictionary can terminate the whole process at once, and
`try`/`catch` cannot intercept that crash. When several goblins must work on
the same data, keep one owner that mutates it and let the others send requests
or results over a channel, as in the examples below.

`goblin build-exe --race` compiles the executable with Go's race detector:
when a race actually occurs at run time, the program reports both racing
operations with stack traces and exits with a failure status. The interpreter
has no equivalent switch — race instrumentation happens when Go code is
compiled — so a program that only ever ran under `goblin run` may hide a race
that the compiled backend hits. Build the racy suspect with `--race` and
exercise it when in doubt.

## Channels

`Chan()` creates an unbuffered channel: a send waits until some goroutine
receives it, and a receive waits until a value is sent. `Chan(capacity = n)`
creates a buffer that can hold up to `n` values before sends block.

| Operation | Behavior |
| --- | --- |
| `channel.send(value, timeout = nil)` | Blocks until a receiver is ready or buffer space exists |
| `channel.receive(timeout = nil)` | Blocks until a value is available |
| `channel.close()` | Prevents future sends; buffered values can still be received. Returns `nil`; closing a closed channel does nothing |
| `for value in channel` | Receives values until the channel is closed and drained |
| `Chan(capacity = 0)` | Requires a non-negative integer |

Sending on a closed channel, or calling `receive()` after a channel is closed
and drained, raises ValueError. Closing a channel twice is allowed, as for a
File, so `defer ch.close()` can stay in place beside an earlier explicit
close.

A channel is iterable, and closing it is how a sender says the stream is over:
a `for` loop receives one value per iteration and ends when the channel is
closed and every buffered value has been received. Several loops iterating the
same channel at once each receive different values.

~~~goblin
var words = Chan(capacity = 16)
spawn(func() {
    defer words.close()
    for word in ["alpha", "beta", "gamma"] {
        words.send(word)
    }
})
for word in words {
    print(word)
}
~~~

Values from several channels are combined by forwarding them into one channel
(a goblin per source, each looping over its own channel); there is no `select`.

## Buffering a known number of results

Use a buffered channel when several workers can finish before the caller starts
receiving. The buffer capacity here matches the number of results.

~~~goblin
func square(value, result) {
    result.send(value * value)
}

var results = Chan(capacity = 3)
for value in [2, 3, 4] {
    spawn(square, value, results)
}

var total = 0
for ignored in range(3) {
    total = total + results.receive()
}
results.close()
print(total) # 29
~~~

An unbuffered `results` channel also works in this example because the caller
begins receiving after it starts the workers. Buffering changes *when* sends
block; it does not guarantee a result order. Do not rely on spawned work
finishing in the order it was started. When each job has a single result,
returning it and calling `wait()` on every handle is simpler still.

## Returning errors explicitly

`wait()` re-raises a goblin's error at the call site, which covers the common
case of one job whose failure the caller handles. When workers stream results
through a channel instead, the caller receives values, not errors. Catch the
error in the worker and send a result record when the caller needs to handle
failures.

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
var outcome = result.receive()
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
print(messages.receive())
messages.close()
~~~

Goblin has no `select` operation and no cancellation primitive. Design each
concurrent operation so every blocking send has a receiver, every expected
result is received, and the owner can decide when it is safe to close its
channel; where waiting forever would be a bug, give the operation a
`timeout`.
