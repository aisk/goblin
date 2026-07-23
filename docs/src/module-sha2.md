# sha256 and sha512

The `sha256` and `sha512` modules expose Go's fixed-size SHA-2 sum functions.
Each function accepts `Bytes` or `Str` and returns raw digest `Bytes`.

| Goblin function | Go equivalent |
| --- | --- |
| `sha256.sum256(data)` | `sha256.Sum256` |
| `sha256.sum224(data)` | `sha256.Sum224` |
| `sha512.sum512(data)` | `sha512.Sum512` |
| `sha512.sum384(data)` | `sha512.Sum384` |
| `sha512.sum512_224(data)` | `sha512.Sum512_224` |
| `sha512.sum512_256(data)` | `sha512.Sum512_256` |

Use `hex.encode_to_string` or `base64.encode` when a textual digest is needed.
SHA-2 hashes do not authenticate data or securely store passwords by themselves.
