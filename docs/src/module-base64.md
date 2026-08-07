# base64

The base64 module converts text or bytes to Base64 text and decodes Base64 text
back to Bytes. The alphabet (standard or URL-safe) and padding are independent
keyword arguments.

~~~goblin
import "base64"

var encoded = base64.encode("hello")
print(encoded)
print(base64.decode(encoded).decode())
~~~

## API

| Function | Result | Description |
| --- | --- | --- |
| `encode(data, url=false, padding=true)` | str | Encode a str or Bytes value |
| `decode(value, url=false, padding=true)` | Bytes | Decode Base64 text |

Set `url=true` for the URL-safe alphabet and `padding=false` to omit `=`
padding; the two compose freely (JWT-style tokens use both). decode() raises
ParseError when the input is malformed, and returns Bytes because Base64 can
represent arbitrary binary data; call `.decode()` on the result only when the
decoded bytes are known to contain UTF-8 text.

~~~goblin
var token = base64.encode(Bytes([251, 255]), url=true, padding=false)
print(token)
print(base64.decode(token, url=true, padding=false))
~~~

Base64 is an encoding, not encryption. Do not use it to conceal passwords,
tokens, or other secrets.
