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

## Strings and bytes

Strings are immutable Unicode text. They can be indexed and iterated by
character, combined with +, and repeated with *. See [Strings](./strings.md)
for conversions and typical methods.

Bytes are immutable raw byte sequences. Bytes("ABC") has size 3 and its first
element is the integer 65. Common byte methods mirror string operations:
decode(), contains(), has_prefix(), split(), replace(), and trim(). Use Bytes
for raw data and strings for text.

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
