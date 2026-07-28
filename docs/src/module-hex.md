# hex

The `hex` module follows Go's `encoding/hex` package.

| Function | Go equivalent |
| --- | --- |
| `encode(data)` | `hex.EncodeToString` |
| `decode(s)` | `hex.DecodeString` |
| `dump(data)` | `hex.Dump` |

Encoding accepts `Bytes` or `Str` and returns lowercase hexadecimal text.
Decoding returns `Bytes` and raises `ParseError` for malformed input.
