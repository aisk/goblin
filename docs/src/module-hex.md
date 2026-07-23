# hex

The `hex` module follows Go's `encoding/hex` package.

| Function | Go equivalent |
| --- | --- |
| `encode_to_string(data)` | `hex.EncodeToString` |
| `decode_string(s)` | `hex.DecodeString` |
| `dump(data)` | `hex.Dump` |

Encoding accepts `Bytes` or `Str` and returns lowercase hexadecimal text.
Decoding returns `Bytes` and raises `ParseError` for malformed input.
