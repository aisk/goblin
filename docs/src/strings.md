# Strings

Strings are immutable Unicode text values written with double quotes. Escape a
double quote or a backslash with a backslash.

~~~goblin
var message = "say: \"hello\""
print(message)        # say: "hello"
print(message.size) # 12
~~~

## Interpolation

Inside a double-quoted string, `{expression}` is replaced by the value of the
expression, as the same text `print` shows for it: a String as it is, a number
or list as it prints, a custom type through its `Show` impl.

~~~goblin
var user = "Ada"
var inbox = ["hi", "re: hi"]
print("{user} has {inbox.size} messages")  # Ada has 2 messages
print("first: {inbox[0]}, all: {inbox}")   # first: hi, all: ["hi", "re: hi"]
print("{user.upper()} {inbox.size * 10}%") # ADA 20%
~~~

The parts are evaluated left to right when the string is. An error raised
while evaluating or showing one is reported at the string literal.

- Write `\{` for a literal `{`. A `}` outside an interpolation is an ordinary
  character, and `\}` also works.
- The expression ends at the `}` matching its `{`, counting `()`, `[]` and
  `{}`, so `"{xs[1:2]}"` and `"{f(g(x))}"` work.
- It cannot contain a `"`, a backslash or a line break: the string would end,
  or the escape would belong to the string. Assign such a value to a variable
  first, or use a raw string inside: `` "{names.join(`, `)}" ``.
- `{}` with nothing inside is an error (write `\{}` for the text `{}`).
- A `:` outside any bracket ends the expression and starts a format
  specifier: see [Formatting](#formatting).

A string without any `{expression}` is a plain constant.

## Formatting

`{expression:spec}` formats the value under a specifier: it pads text to a
width and controls how numbers are written.

~~~goblin
var price = 3.1
var name = "tea"
print("{name:<8}|{price:>8.2f}|")      # tea     |    3.10|
print("{255:x} {255:08b} {7:03}")      # ff 11111111 007
print("{1234567:,} {0.5:+.1f}")        # 1,234,567 +0.5
print("{name:*^9}")                    # ***tea***
~~~

A specifier is, in this order and each part optional:

~~~
[[fill] align] [+] [0] [width] [,] [.precision] [type]
~~~

| Part | Meaning |
| --- | --- |
| width | Minimum number of characters; longer text is never cut |
| align | `<` left, `>` right, `^` centre (an odd space goes right). Numbers align right by default, everything else left |
| fill | The padding character, default a space. Only before an explicit align: `*^9`, ` >5` |
| `+` | Show `+` on numbers that are not negative |
| `0` | Pad a number with zeros between its sign and its digits (`{-42:08}` is `-0000042`); not together with an align |
| `,` | Group the integer digits in threes: an Int with no type or `d`, or a number with `f` |
| `.precision` | Digits after the decimal point; needs `f` or `e` (`{x:.2}` is an error, write `{x:.2f}`) |
| `d` | Decimal Int, the default |
| `x` `X` `b` `o` | Int in hexadecimal (lower or upper case), binary or octal, with no prefix: write `"0x{n:04x}"` |
| `f` | Fixed point, 6 decimals unless a precision is given; an Int is converted |
| `e` | Scientific (`1.50e+03`), 6 decimals unless a precision is given; an Int is converted |

An Int takes every part. A Float takes all but the Int types; with no type it
shows the text `print` shows (`2.0`, `1e+16`), and `,` on it needs `f`
(`{x:,.2f}`). NaN and the infinities show as `print` shows them, padded with
spaces. Any other value (a String, Bool, nil, a list, a custom type through
its `Show` impl) takes only fill, align and width, applied to its `print`
text.

- The specifier is literal text: it is not interpolated and takes no escapes,
  so it cannot contain `{`, `}`, `"`, a backslash or a line break. Spaces in it
  count (`{x: >5}` pads with spaces); spaces before the `:` belong to the
  expression.
- A malformed specifier (`{x:}`, `{x:5q}`, `{x:<05}`) is a syntax error,
  reported where it stands when the program is parsed.
- A value whose type does not take the specifier is a TypeError when the
  string is evaluated: `format specifier ".2f" needs an Int or Float, got
  String`.
- Rounding is to the nearest value of the exact binary number, so
  `{2.675:.2f}` is `2.67`, as it is in Go and Python.

When the specifier is only known at run time, such as a column width
computed from the data, use the built-in `format(value, spec)`, which is
exactly what `"{value:spec}"` runs:

~~~goblin
var names = ["tea", "coffee", "hot chocolate"]
var width = max(names.map(func(n) { return n.size }))
for name in names {
    print(format(name, "<{width}") + "|")
}
~~~

## Escapes

| Escape | Meaning |
| --- | --- |
| `\n` `\t` `\r` | Newline, tab, carriage return |
| `\"` `\\` | Double quote, backslash |
| `\{` `\}` | Literal braces |
| `\0` | The NUL character |
| `\xHH` | The character U+00HH, exactly two hex digits (`"\x1b[31m"` starts red terminal text) |
| `\u{H...}` | The character with that code point, one to six hex digits (`"\u{1F600}"`) |

`\xHH` is a character, not a raw byte: `"\xe9"` is `"é"`, two bytes in
UTF-8. Use Bytes for byte values. Any other escape is a syntax error, and so
is a `\u{...}` that is not a valid Unicode code point.

## Raw strings

A string between backticks is raw: backslashes and braces are ordinary
characters, and it may span lines. It cannot contain a backtick. Raw strings
suit regular expressions, Windows paths and embedded JSON.

~~~goblin
print(`C:\temp\{name}`)       # C:\temp\{name}
var digits = `\d+(\.\d+)?`
var config = `{"port": 8080}`
var text = `first line
second line`
print(text.size)               # 22
~~~

Line breaks in a raw string are always `\n`, even in a file saved with CRLF
line endings.

size counts Unicode characters, not bytes; it is a property, not a method.
Strings iterate by character, and `text[i]` is the character at position `i`
as a one-character string. A negative position counts from the end, and one
outside the string raises IndexError. Strings are immutable, so `text[i] = c`
is a TypeError.

~~~goblin
var language = "Goblin"

for character in language {
    print(character)
}
print(language[0], language[-1]) # G n
~~~

## Slicing

`text[low:high]` returns the characters from position `low` up to, but
excluding, `high`. Positions count characters, as size does, and either bound
may be omitted.

~~~goblin
var text = "héllo, wörld"
print(text[:5])  # héllo
print(text[7:])  # wörld
print(text[1:2]) # é
~~~

Bounds are positions between characters, a negative one counting from the end
(`text[:-1]` drops the last character). As for a
[list slice](./collections.md#slicing), a bound outside the text stops at its
edge, so `line[:80]` is at most 80 characters and never raises.

## Characters and code points

`c.code_point` is the Unicode code point of a string's first character, as an
Int, and nil for the empty string. Indexing or iterating a string gives
one-character strings, which is where it is used. In the other direction,
`n.to_char()` is the one-character string for the code point `n`; it raises
ValueError for a number outside `0` to `0x10FFFF` or a surrogate
(`0xD800`–`0xDFFF`).

~~~goblin
print("A".code_point)      # 65
print("é".code_point)      # 233
print("".code_point)       # nil
print((97).to_char())      # a
print(0x1F600.to_char())   # 😀

func rot13(c) {
    if c >= "a" && c <= "z" {
        return ((c.code_point - 97 + 13) % 26 + 97).to_char()
    }
    return c
}
print(List("hello").map(rot13).join(""))   # uryyb
~~~

A decimal integer literal needs parentheses before a method, `(97).to_char()`:
written `97.to_char()` it reads as the start of a float. A variable or a
hexadecimal literal needs none.

## Combining and converting text

Use + to concatenate two strings. Both operands must be strings: `"item-" + 3`
raises TypeError, because text that only looks like a number, such as a port
read from the environment, would otherwise turn `port + 1` into `"80801"`
without an error. Put the value in an interpolation or convert it with
`String(x)` first. `*` repeats a string by an integer count, in either operand
order; a negative count raises ValueError.

~~~goblin
print("go" + "blin")      # goblin
print("item-{3}")         # item-3
print("item-" + String(3)) # item-3
print("ha" * 3)           # hahaha
print(3 * "ha")           # hahaha
~~~

String(value) converts any value to its display text, the same text `print`
shows and interpolation inserts. Interpolation is usually the clearer way to
build a string from other values.

~~~goblin
var port = 8080
print("port=" + String(port)) # port=8080
print("port={port}")       # port=8080
~~~

Bytes become text in two ways: `data.decode()` reads them as UTF-8 text, and
`data.to_hex()` spells each byte as two lower-case hexadecimal digits, which is
how a digest or a random token is usually printed.

~~~goblin
print(Bytes([104, 105]).decode()) # hi
print(Bytes([0, 255]).to_hex())   # 00ff
~~~

## Common string methods

| Method | Purpose |
| --- | --- |
| size | Character count (property) |
| upper() / lower() | Change letter case |
| contains(value) | Test for a substring |
| has_prefix(prefix) / has_suffix(suffix) | Test the beginning or end |
| index(value, start=0) | Character position of a substring, or nil |
| count(value) | Count non-overlapping occurrences |
| replace(old, new, count=nil) | Replace occurrences, at most `count` of them |
| split(sep=nil, count=nil) | Split into a list, at most `count` times |
| lines() | The lines of the text, without their line endings |
| cut(sep) | Split once: `[before, after]`, or `[text, nil]` |
| trim(chars=nil) / trim_start(chars=nil) / trim_end(chars=nil) | Trim both ends, the start, or the end |
| trim_prefix(prefix) / trim_suffix(suffix) | Remove one matching edge |

Methods accept named arguments as well as positional ones.

~~~goblin
var title = "  Goblin book  "
print(title.trim().upper())
print("a,b,c".split(sep=",", count=1))   # ["a", "b,c"]
print("one one one".replace(old="one", new="two", count=2))
print("config.toml".trim_suffix(".toml"))
~~~

index() counts characters, like `text[i]`, and returns nil when the substring
is absent; `start` is a position from `-size` to `size`. Position `0` is false
in a condition, so test the result with `!= nil`, or use contains().

`count` on replace() and split() is how many times the operation runs; leaving
it out (or passing nil) means no limit, and a negative count raises
ValueError. So `split(",", count=1)` gives at most two pieces. split() without
a separator splits at runs of Unicode whitespace and drops empty pieces:
`" a  b ".split()` is `["a", "b"]`; with a count, the rest of the text after
the last split, leading whitespace removed, is the final piece. An empty
separator raises ValueError.

lines() is how text becomes its lines: a line ends at `\n` or `\r\n` (a lone
`\r` does not end one), the endings are removed, text that ends with a line
ending has no empty last line, and `""` has no lines at all. That makes it the
right call for the output of a command or the body of a file read whole,
where `split("\n")` would leave a last `""`:

~~~goblin
print("a\nb\n".lines())        # ["a", "b"]
print("a\r\n\nb".lines())      # ["a", "", "b"]
print("".lines())               # []
print("a\nb\n".split("\n"))    # ["a", "b", ""]
~~~

cut() returns two values, so it destructures:

~~~goblin
var key, value = "port=80".cut("=")
print(key, value)               # port 80
print("port".cut("="))          # ["port", nil]
~~~

trim() with no argument removes Unicode whitespace. Supplying `chars` trims
any of its characters, not an exact substring; trim_start() and trim_end() do
the same at one end. Use trim_prefix() or trim_suffix() to remove an exact
substring.

To repeat text use `*`; to find the last occurrence of a separator, split and
take `.last`, or use the [regex](./module-regex.md) module; to split at
whitespace use split() without a separator.

## Common text-processing patterns

Use trim() before validating user-supplied text, split() to turn a delimited
setting into a list, and replace() when normalizing a known spelling or format.

~~~goblin
var raw_tags = " go, language,tools "
var tags = []
for tag in raw_tags.split(",") {
    tags.push(tag.trim())
}
print(tags)

var filename = "report.TXT"
if filename.lower().has_suffix(".txt") {
    print("text file")
}
~~~

Use contains() when a yes/no answer is enough, and index() when you need the
position.
