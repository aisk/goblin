# pem

The pem module encodes and decodes PEM blocks using Go's `encoding/pem`
package.

~~~goblin
import "pem"

var block = pem.Block("MESSAGE", "Goblin", {"Source": "example"})
var encoded = block.encode()
var result = pem.decode(encoded)
print(result[0].label)
print(result[1])
~~~

## Block

`Block(label, data, headers={})` constructs a block. `data` accepts str or
Bytes and header keys and values must be strings.

| Member | Type | Description |
| --- | --- | --- |
| `label` | str | PEM block label such as `CERTIFICATE` |
| `data` | Bytes | Decoded block contents |
| `headers` | dict | Optional PEM headers |
| `encode()` | Bytes | Encode the block, including delimiters |

The member is named `label`, rather than Go's `Block.Type`, because `type` is a
Goblin language keyword.

`decode(data)` returns `[block, rest]`. `block` is nil when no PEM block was
found; `rest` contains all bytes after the decoded block, allowing repeated
decoding of concatenated input.
