# Built-in types

Goblin values have runtime types; variables do not need type annotations. An
operation succeeds only when its value types are compatible.

| Type | Examples | Notes |
| --- | --- | --- |
| Int | 0, -42 | Signed 64-bit whole number |
| Float | 3.14, -0.5 | 64-bit floating-point number |
| Bool | true, false | Logical value |
| Nil | nil | Absence of a value; it prints as nil |
| String | "hello" | Immutable Unicode text |
| Bytes | Bytes("data") | Immutable raw byte sequence |
| List | [1, "two"] | Ordered, mutable collection |
| Dict | {"name": "Ada"} | Mutable key/value collection, in insertion order |
| Chan | Chan() | Channel for concurrent functions |
| Goblin | spawn(func() {}) | Handle to a function running concurrently |
| Function | func() {} | Callable value |

Custom types are covered in [Types and methods](./types.md).

## Numbers

Integer literals have no decimal point and can be written in decimal,
hexadecimal, octal or binary (`255`, `0xFF`, `0o377`, `0b1111_1111`); float
literals have a decimal point or an exponent (`2.5`, `1e9`). See
[Syntax](./syntax.md) for the exact rules. Arithmetic preserves
an integer result when both operands are integers. If either operand is a
float, the result is a float. Integer division truncates its fractional part.
The `%` operator returns the remainder after truncating division; the result
has the dividend's sign and is a float if either operand is a float.

~~~goblin
print(7 / 2)     # 3
print(7 / 2.0)   # 3.5
print(2 + 0.5)   # 2.5
print(-3 * 4)    # -12
print(-7 % 3)    # -1
print(7.5 % 2)   # 1.5
print(4 / 2.0)   # 2.0
~~~

A Float always prints as a float: a whole value keeps a `.0`, and very large or
very small magnitudes use an exponent (`1e+16`, `1e-05`). To choose the
number of decimals, format it: `"{2 / 3.0:.2f}"` is `0.67` (see
[Formatting](./strings.md#formatting)).

Integers are 64-bit and Goblin has no big integers. Arithmetic whose result
does not fit raises OverflowError instead of wrapping around, and so does
converting a float or string that is out of range with Int().

Numbers can be compared across integer and float values. Division or modulo by
zero raises ZeroDivisionError. Int() and Float() convert numbers, booleans,
and numeric strings; converting a float to Int removes its fractional portion.

~~~goblin
print(Int("42"))   # 42
print(Int(3.9))     # 3
print(Float(true))  # 1.0
print(max(3, 5, 4))
~~~

### When to convert and when to calculate

Use Int() or Float() at the edge of a program, where a value arrives as text
or where an integer operation must become floating-point. Keep calculations in
their natural numeric form after that. For example, parse a configuration value
once, then use min() and max() to keep it within a permitted range.

~~~goblin
var requested_workers = Int("12")
var workers = min(max(requested_workers, 1), 8)
print(workers) # 8
~~~

Int() rejects non-numeric text with ValueError. This makes it suitable for
validating numeric input inside a try/catch block.

## Booleans and nil

Use Bool(value) to convert any value by truthiness. False, nil, numeric zero,
and empty strings, lists, dictionaries, or bytes are false.

~~~goblin
print(Bool(""))       # false
print(Bool([1]))      # true
print(!nil)           # true
print(true && false)  # false
~~~

A function with no explicit result returns nil. Comparing any value against
nil with `==` is safe — it is true only for nil itself — but arithmetic and
indexing on nil are errors.

### Using truthiness for optional values

Truthiness is convenient for choosing a fallback or guarding an optional
collection. Use an explicit comparison with nil when zero, false, or an empty
collection must still be treated as a present value.

~~~goblin
var nickname = ""
print(nickname || "anonymous") # anonymous

var limit = 0
if limit == nil {
    print("no limit supplied")
}
~~~

## Strings and bytes

Strings are immutable Unicode text. They can be iterated by character,
indexed with `text[i]` (giving a one-character string), combined with +,
repeated with *, and sliced with `text[low:high]`.
Use `index()` to find a character position. See
[Strings](./strings.md) for conversions and typical methods.

Bytes are immutable raw byte sequences. Bytes("ABC") has size 3 and its first
element is the integer 65. Like list indexes, a negative byte index counts
from the end, `bytes[low:high]` slices them, and `*` repeats them. Bytes share
the subsequence methods of strings, with positions counted in bytes:
contains(), count(), index(), has_prefix(), has_suffix(), trim_prefix(),
trim_suffix(), replace(), split() and cut(), each taking a String or Bytes
argument. Text operations (upper(), trim(), splitting at whitespace) belong to
strings: decode() first, which raises ValueError for invalid UTF-8. to_hex()
shows Bytes as text of two lower-case hexadecimal digits per byte (empty Bytes
give ""), the usual way to print a digest or a random token. There is no
reverse conversion from hexadecimal text to Bytes. Bytes are hashable, so they
can be dictionary keys. Use Bytes for raw data and strings for text.

### Working with bytes

Use Bytes when reading or sending binary-oriented data, or when indexing must
produce numeric byte values. Use decode() when that data should become text.

~~~goblin
var header = Bytes("GET")
print(header[0])          # 71
print(header.contains("E"))
print(header.decode())    # GET
print(header.to_hex())    # 474554
~~~

## Lists and dictionaries

Lists and dictionaries are mutable collections. A list is ordered and uses
integer indexes; a dictionary maps keys to values. Empty collections are false
in conditions. See [Collections](./collections.md) for creation, indexing, and
method guides.

## Channels

Chan(capacity=0) creates a channel. Send with send(), receive with receive(),
and close with close(); send() and receive() take an optional timeout= in
seconds. Capacity 0 creates an unbuffered channel. Use spawn() to start a
concurrent function.

~~~goblin
var done = Chan()
spawn(func() {
    done.send("finished")
})
print(done.receive())
done.close()
~~~

### Coordinating concurrent work

An unbuffered channel makes send() wait until another function calls
receive(). A buffered channel can accept up to its capacity before a receiver
is ready. A for loop over a channel receives values until it is closed and
drained.

~~~goblin
var results = Chan(capacity = 2)
spawn(func() {
    defer results.close()
    results.send(2 * 2)
    results.send(3 * 3)
})
var total = 0
for value in results {
    total = total + value
}
print(total) # 13
~~~

Close a channel only when no more values will be sent. Calling receive() on a
closed, drained channel raises ValueError, rather than producing a special nil
value.

## Goblin handles

spawn(function, args...) starts the function in a new goroutine and returns a
Goblin handle; there is no Goblin constructor. wait() joins it: the function's
result is returned, an error it raised is re-raised, and the outcome is cached
across repeated calls. The done property reports completion without blocking,
and wait(timeout = seconds) raises TimeoutError when the function outlives the
timeout. An error no wait() ever saw is reported on stderr when the program
ends.

~~~goblin
func square(value) {
    return value * value
}

var worker = spawn(square, 6)
print(worker.wait()) # 36
~~~

See [Concurrency](./concurrency.md) for how handles, channels, and spawn fit
together.

## Common operations

| Type | Common constructors and operations |
| --- | --- |
| Int / Float | Int(value), Float(value), max(...), min(...) |
| Bool / Nil | Bool(value), !value, value && other, value \|\| other |
| String | String(value), size, contains(), split(), replace() |
| Bytes | Bytes(value), size, decode(), to_hex(), contains(), split() |
| List | List(value), size, first, last, push(), pop(), sort(), copy() |
| Dict | Dict(), get(), get_or_set(), remove(), keys(), items(), update() |
| Chan | Chan(capacity), send(value), receive(), close(), for value in channel |
| Goblin | spawn(function, args...), wait(), done, wait_any(goblins) |
| Function | Function(value), value(...), name |

Use `attributes(value)` in the REPL to inspect all operations provided by a
runtime value, and `type_of(value)` for its type; see
[Built-in functions](./built-in-functions.md#constructors-and-type-identity).
