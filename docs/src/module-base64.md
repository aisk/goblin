# base64

The base64 module turns bytes into Base64 text and back: Basic authentication
headers, data URLs, binary data inside JSON, the parts of a JWT.

## Common tasks

~~~goblin
import "base64"
import "fs"
import "json"

# Basic authentication
var credentials = base64.encode("{user}:{password}")
var headers = {"Authorization": "Basic {credentials}"}

# Binary data inside JSON
var payload = json.encode({"name": "logo.png", "data": base64.encode(fs.read_bytes("logo.png"))})

# And back
var image = base64.decode(json.decode(payload)["data"])
fs.write("logo-copy.png", image)

# The middle part of a JWT: URL alphabet, no padding
var claims = json.decode(base64.decode(token.split(".")[1], url_safe=true).decode())
~~~

`examples/base64.goblin` runs each of these tasks.

## Module members

| Member | Returns | Behaviour |
| --- | --- | --- |
| `encode(data, url_safe=false, padding=true)` | String | Base64 text of `data` (a String's UTF-8 bytes, Bytes, or a readable read to its end). `url_safe=true` uses `-` and `_` instead of `+` and `/`. `padding=false` leaves off the trailing `=` |
| `decode(text, url_safe=false)` | Bytes | The bytes `text` (a String) encodes. Trailing `=` padding may be present or absent, but when present it must be complete. Line breaks inside `text` are ignored. `""` decodes to empty Bytes |

`decode` returns Bytes because Base64 carries binary data; call `.decode()` on
the result when it is known to be text.

### Errors

| Failure | Raises |
| --- | --- |
| `text` is not valid Base64 in the chosen alphabet | ParseError `decode() invalid Base64 at position 5`, the position being the character index in `text` |
| `data` or `text` has the wrong type | TypeError |
| Reading a readable `data` fails | the error the readable raised, such as IOError |

## Left out

Base32, Ascii85 and quoted-printable encodings.
