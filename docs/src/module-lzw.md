# lzw

The lzw module wraps Go's `compress/lzw` package as whole-value operations.
It deliberately does not expose Go readers and writers.

~~~goblin
import "lzw"

var compressed = lzw.compress("Goblin data")
print(lzw.decompress(compressed))
~~~

| Function | Returns | Description |
| --- | --- | --- |
| `compress(data, order="lsb", lit_width=8)` | Bytes | Compress str or Bytes data |
| `decompress(data, order="lsb", lit_width=8)` | Bytes | Decompress a complete LZW value |

`order` is `lsb` or `msb`; `lit_width` must be from 2 through 8. Both options
must match the format being read. Invalid compressed input raises ParseError.
