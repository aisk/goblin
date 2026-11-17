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
- `{}` with nothing inside is an error (write `\{}` for the text `{}`), and
  so is a `:` outside any bracket, as in `"{x:5}"`: format specifiers are not
  supported. Format numbers with their methods or a module before
  interpolating them.

A string without any `{expression}` is a plain constant.

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

Bounds are positions between characters, from `-size` to `size`, a negative
one counting from the end (`text[:-1]` drops the last character); anything
else raises IndexError, exactly as for a [list slice](./collections.md#slicing).

## Combining and converting text

Use + to concatenate strings. A string can also concatenate an integer or
boolean, and * repeats it by an integer count, in either operand order. A
negative count raises ValueError.

~~~goblin
print("go" + "blin") # goblin
print("item-" + 3)   # item-3
print("ha" * 3)      # hahaha
print(3 * "ha")      # hahaha
~~~

String(value) converts any value to its display text, the same text `print`
shows and interpolation inserts. Interpolation is usually the clearer way to
build a string from other values.

~~~goblin
var port = 8080
print("port=" + String(port)) # port=8080
print("port={port}")       # port=8080
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
take `.last`, or use the [regexp](./module-regexp.md) module; to split at
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
