# Built-in types

Goblin supplies a small set of built-in runtime types. Values have no static
type annotation; the operation determines whether their types are compatible.
For example, an integer and a float can be added, but adding a dictionary to a
number is an error.

| Type | Examples | Notes |
| --- | --- | --- |
| Integer | `0`, `-42` | Signed whole number |
| Float | `3.14`, `-0.5` | Floating-point number |
| Bool | `true`, `false` | Logical value |
| Nil | `nil` | Absence of a value; it prints as `none` |
| String | `"hello"` | Immutable Unicode text |
| Bytes | `Bytes("data")` | Immutable raw byte sequence |
| List | `[1, "two"]` | Ordered, mutable collection |
| Dict | `{"name": "Ada"}` | Mutable key/value collection |
| Chan | `Chan(0)` | Channel for communicating between spawned functions |

Custom types are covered in [Types and methods](./types.md).

## Numbers

Integer literals have no decimal point; float literals do. Arithmetic preserves
an integer result when both operands are integers. If either operand is a
float, the result is a float. Integer division truncates its fractional part.

```goblin
print(7 / 2)     # 3
print(7 / 2.0)   # 3.5
print(2 + 0.5)   # 2.5
print(-3 * 4)    # -12
```

Numbers may be compared across integer and float values. Division by zero
raises `ZeroDivisionError`.

`Int()` and `Float()` explicitly convert an integer, float, boolean, or a
numeric string. Converting a float to `Int` removes the fractional portion.

```goblin
print(Int("42"))   # 42
print(Int(3.9))     # 3
print(Float(true))  # 1
```

`max()` and `min()` accept one or more numeric arguments. A result is a float
when any supplied argument is a float.

## Booleans and `nil`

Booleans are written as `true` and `false`. Use `Bool(value)` to convert any
value by its truthiness.

```goblin
print(Bool(""))       # false
print(Bool([1]))      # true
print(!nil)           # true
print(true && false)  # false
```

`nil` represents the absence of a value. A function with no explicit return
value returns `nil`; printing it produces `none`. It can be compared to another
`nil`, but arithmetic and indexing on it are errors.

## Strings

Strings use double quotes. Escape a double quote or a backslash with a
backslash.

```goblin
var message = "say: \"hello\""
print(message)        # say: "hello"
print(message.size()) # 12
```

`size()` counts Unicode characters, and strings can be indexed and iterated by
character. Strings support concatenation with another string, integer, or
boolean, and may be repeated by an integer.

```goblin
print("go" + "blin") # goblin
print("item-" + 3)    # item-3
print("ha" * 3)       # hahaha
print("Goblin"[0])    # G
```

Useful string methods include `upper()`, `lower()`, `contains()`, `split()`,
`replace()`, `trim()`, `has_prefix()`, and `has_suffix()`.

```goblin
var title = "  Goblin book  "
print(title.trim().upper())
print("a,b,c".split(","))
print("one one".replace("one", "two"))
```

Use `Str(value)` to turn a value into its printed text representation.

## Bytes

`Bytes()` constructs an immutable sequence of raw bytes from a string or from
another byte sequence. It is useful when an API expects binary data rather than
text. Indexing and iteration return integer byte values from 0 through 255.

```goblin
var data = Bytes("ABC")
print(data.size())   # 3
print(data[0])       # 65
print(data.contains("B"))
```

Bytes can be concatenated with `+`; they cannot be modified in place.

## Lists and dictionaries

Lists and dictionaries are mutable collections. A list is ordered and uses
integer indexes; a dictionary maps a key to a value. Empty collections are
false in conditions.

```goblin
var tasks = ["write", "test"]
tasks.push("ship")
tasks[0] = "draft"

var user = {"name": "Ada"}
user["active"] = true
print(user.get("role", default="reader"))
```

Lists also support `pop()`, `first()`, `last()`, `join()`, `reverse()`, and
`copy()`. Dictionaries provide `contains()`, `keys()`, `values()`, `items()`,
`set_default()`, `pop()`, and `update()`. See [Strings and
collections](./collections.md) for their everyday use.

## Channels

`Chan(capacity)` creates a channel. Send with `send()`, receive with `recv()`,
and close it with `close()`. A capacity of `0` makes an unbuffered channel.
Use `spawn()` to start a function concurrently.

```goblin
var done = Chan(0)
spawn(func() {
    done.send("finished")
})
print(done.recv())
done.close()
```
