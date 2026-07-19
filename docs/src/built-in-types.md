# Built-in types

Goblin values have runtime types; variables do not need type annotations. An
operation succeeds only when its value types are compatible.

| Type | Examples | Notes |
| --- | --- | --- |
| Integer | 0, -42 | Signed whole number |
| Float | 3.14, -0.5 | Floating-point number |
| Bool | true, false | Logical value |
| Nil | nil | Absence of a value; it prints as none |
| String | "hello" | Immutable Unicode text |
| Bytes | Bytes("data") | Immutable raw byte sequence |
| List | [1, "two"] | Ordered, mutable collection |
| Dict | {"name": "Ada"} | Mutable key/value collection |
| Chan | Chan(0) | Channel for concurrent functions |

Custom types are covered in [Types and methods](./types.md).

## Numbers

Integer literals have no decimal point; float literals do. Arithmetic preserves
an integer result when both operands are integers. If either operand is a
float, the result is a float. Integer division truncates its fractional part.

~~~goblin
print(7 / 2)     # 3
print(7 / 2.0)   # 3.5
print(2 + 0.5)   # 2.5
print(-3 * 4)    # -12
~~~

Numbers can be compared across integer and float values. Division by zero
raises ZeroDivisionError. Int() and Float() convert numbers, booleans, and
numeric strings; converting a float to Int removes its fractional portion.

~~~goblin
print(Int("42"))   # 42
print(Int(3.9))     # 3
print(Float(true))  # 1
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

A function with no explicit result returns nil. Nil can be compared with nil,
but arithmetic and indexing on it are errors.

### Using truthiness for optional values

Truthiness is convenient for choosing a fallback or guarding an optional
collection. Use an explicit comparison with nil when zero, false, or an empty
collection must still be treated as a present value.

~~~goblin
var nickname = ""
if nickname {
    print(nickname)
} else {
    print("anonymous")
}

var limit = 0
if limit == nil {
    print("no limit supplied")
}
~~~

## Strings and bytes

Strings are immutable Unicode text. They can be indexed and iterated by
character, combined with +, and repeated with *. See [Strings](./strings.md)
for conversions and typical methods.

Bytes are immutable raw byte sequences. Bytes("ABC") has size 3 and its first
element is the integer 65. Common byte methods mirror string operations:
decode(), contains(), has_prefix(), split(), replace(), and trim(). Use Bytes
for raw data and strings for text.

### Working with bytes

Use Bytes when reading or sending binary-oriented data, or when indexing must
produce numeric byte values. Use decode() when that data should become text.

~~~goblin
var header = Bytes("GET")
print(header[0])          # 71
print(header.contains("E"))
print(header.decode())    # GET
~~~

## Lists and dictionaries

Lists and dictionaries are mutable collections. A list is ordered and uses
integer indexes; a dictionary maps keys to values. Empty collections are false
in conditions. See [Collections](./collections.md) for creation, indexing, and
method guides.

## Channels

Chan(capacity) creates a channel. Send with send(), receive with recv(), and
close with close(). Capacity 0 creates an unbuffered channel. Use spawn() to
start a concurrent function.

~~~goblin
var done = Chan(0)
spawn(func() {
    done.send("finished")
})
print(done.recv())
done.close()
~~~

### Coordinating concurrent work

An unbuffered channel makes send() wait until another function calls recv().
This makes it useful for returning one result from spawned work. A buffered
channel can accept up to its capacity before a receiver is ready.

~~~goblin
var results = Chan(2)
spawn(func() { results.send(2 * 2) })
spawn(func() { results.send(3 * 3) })
print(results.recv() + results.recv())
results.close()
~~~

Close a channel only when no more values will be sent. Receiving from a closed,
drained channel raises ValueError, rather than producing a special nil value.

## Common operations

| Type | Common constructors and operations |
| --- | --- |
| Integer / Float | Int(value), Float(value), max(...), min(...) |
| Bool / Nil | Bool(value), !value, value && other, value \|\| other |
| String | Str(value), size(), contains(), split(), replace() |
| Bytes | Bytes(value), size(), decode(), contains(), split() |
| List | List(value), size(), push(), pop(), sort(), copy() |
| Dict | Dict(), get(), set_default(), keys(), items(), update() |
| Chan | Chan(size), send(value), recv(), close() |

Use value.attributes() in the REPL to inspect all operations provided by a
runtime value.
