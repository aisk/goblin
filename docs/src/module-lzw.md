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
| `compress(data, order=lzw.LSB, lit_width=8, dest=unit)` | Bytes or unit | Compress str or Bytes data |
| `decompress(data, order=lzw.LSB, lit_width=8)` | Bytes | Decompress a complete LZW value |

`order` is the `lzw.LSB` or `lzw.MSB` constant; `lit_width` must be from 2
through 8. Both options must match the format being read. Invalid compressed
input raises ParseError.

Passing `dest=` streams the compressed output into any writer object — an
object with a `write(data)` method, such as an open `fs` file — and `compress`
then returns `unit`.
