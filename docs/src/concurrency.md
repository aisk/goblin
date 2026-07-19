# Concurrency

`spawn(function, args...)` starts a Goblin function in a new goroutine.
It returns nil immediately. The spawned function's return value and unhandled
error are discarded, so use a channel when the caller needs a result or a
failure signal.

~~~goblin
var result = Chan()

spawn(func() {
    result.send(21 * 2)
})

print(result.recv()) # 42
result.close()
~~~

## Channels

`Chan()` and `Chan(0)` create unbuffered channels: send waits for a receiver,
and receive waits for a sender. `Chan(size)` buffers up to `size` values.
Sending on a closed channel, closing a channel twice, or receiving from a
closed and drained channel raises ValueError.

The code that knows no more values will be sent should close the channel. Do
not close a channel while a spawned sender may still use it. Goblin has no
select operation, cancellation primitive, or automatic joining of spawned
functions, so design communication so every blocking send has a receiver.
