# base32, ascii85, html, and quotedprintable

These modules adapt Go's value-oriented text encoding packages. They process a
complete value and do not expose Go readers or writers.

## base32

`base32` wraps `encoding/base32`.

| Function | Result |
| --- | --- |
| `encode(data, padding=true)` | Encode `Bytes` or `Str` with the standard alphabet |
| `decode(data, padding=true)` | Decode a string to `Bytes` |
| `hex_encode(data, padding=true)` | Encode with the extended-hex alphabet |
| `hex_decode(data, padding=true)` | Decode the extended-hex alphabet to `Bytes` |

Set `padding=false` on both encoding and decoding to use the unpadded form.
Malformed input raises `ParseError`.

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
