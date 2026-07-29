# hex

The `hex` module follows Go's `encoding/hex` package.

| Function | Go equivalent |
| --- | --- |
| `encode(data)` | `hex.EncodeToString` |
| `decode(s)` | `hex.DecodeString` |
| `dump(data)` | `hex.Dump` |

Encoding accepts `Bytes` or `Str` and returns lowercase hexadecimal text.
Decoding returns `Bytes` and raises `ParseError` for malformed input.

~~~goblin
import "hex"

var encoded = hex.encode(Bytes("Goblin"))
print(encoded)                    # 476f626c696e
print(hex.decode(encoded).decode()) # Goblin
print(hex.dump(Bytes("Goblin")))
~~~

Use `encode()` for compact machine-readable text. `dump()` instead produces a
multi-line, offset-labelled representation intended for diagnostics. An odd
number of hexadecimal digits or a non-hexadecimal character makes `decode()`
raise `ParseError`; it never returns a partial result.
