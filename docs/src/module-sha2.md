# sha256 and sha512

The `sha256` and `sha512` modules expose Go's fixed-size SHA-2 functions. Every
function accepts `Bytes` or `Str`. `sum` functions return raw digest `Bytes`;
the corresponding `hex` functions return lowercase hexadecimal text.

| Goblin function | Go equivalent |
| --- | --- |
| `sha256.sum(data)` / `sha256.hex(data)` | `sha256.Sum256` |
| `sha256.sum224(data)` | `sha256.Sum224` |
| `sha256.hex224(data)` | `sha256.Sum224`, as text |
| `sha512.sum(data)` / `sha512.hex(data)` | `sha512.Sum512` |
| `sha512.sum384(data)` | `sha512.Sum384` |
| `sha512.hex384(data)` | `sha512.Sum384`, as text |
| `sha512.sum224(data)` / `sha512.hex224(data)` | `sha512.Sum512_224` |
| `sha512.sum256(data)` / `sha512.hex256(data)` | `sha512.Sum512_256` |

Use a module's `hex` variant for hexadecimal text or `base64.encode` for
Base64 text.
SHA-2 hashes do not authenticate data or securely store passwords by themselves.

~~~goblin
import "sha256"
import "sha512"

var digest = sha256.sum("Goblin")
print(digest.size())       # 32
print(sha256.hex("Goblin"))
print(sha512.sum384("Goblin").size()) # 48
~~~

Use raw `Bytes` when a binary format has a fixed digest field, and use a `hex`
function when a textual protocol explicitly expects hexadecimal. For message
authentication or password storage, use a purpose-built construction rather
than a bare hash; Goblin does not currently provide one in its standard
library.
