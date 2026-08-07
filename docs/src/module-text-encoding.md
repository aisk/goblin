# base32, ascii85, html, and quotedprintable

These modules adapt Go's value-oriented text encoding packages. They process a
complete value and do not expose Go readers or writers.

## base32

`base32` wraps `encoding/base32`.

| Function | Result |
| --- | --- |
| `encode(data, hex=false, padding=true)` | Encode `Bytes` or `Str` |
| `decode(data, hex=false, padding=true)` | Decode a string to `Bytes` |

Set `hex=true` to use the extended-hex alphabet and `padding=false` for the
unpadded form; the two keywords compose freely and must match between
encoding and decoding. Malformed input raises `ParseError`.

## ascii85

`ascii85.encode(data)` accepts `Bytes` or `Str` and returns encoded text.
`ascii85.decode(data)` accepts encoded text and returns `Bytes`. Whitespace in
encoded input follows Go's `encoding/ascii85` rules. Malformed input raises
`ParseError`.

## html

`html.escape(s)` replaces the five characters significant in HTML text with
entities. `html.unescape(s)` resolves named and numeric HTML entities. Both
accept and return `Str` and mirror `html.EscapeString` and
`html.UnescapeString`; this module does not parse or sanitize HTML documents.

## quotedprintable

`quotedprintable.encode(data)` accepts `Bytes` or `Str` and returns encoded
text. `quotedprintable.decode(data)` accepts `Bytes` or `Str` and returns
decoded `Bytes`. The module wraps Go's `mime/quotedprintable` package using an
internal buffer so no writer object appears in the Goblin API. Invalid input
raises `ParseError`.
