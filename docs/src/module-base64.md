# base64

The base64 module converts text or bytes to Base64 text and decodes Base64 text
back to Bytes. It supports both the standard alphabet and the unpadded,
URL-safe alphabet.

~~~goblin
import "base64"

var encoded = base64.encode("hello")
print(encoded)
print(base64.decode(encoded).decode())
~~~

## API

| Function | Result | Description |
| --- | --- | --- |
| `encode(data)` | str | Encode a str or Bytes value with standard padded Base64 |
| `decode(value)` | Bytes | Decode standard padded Base64 text |
| `url_encode(data)` | str | Encode with the URL-safe alphabet and omit padding |
| `url_decode(value)` | Bytes | Decode unpadded URL-safe Base64 text |

decode() and url_decode() raise ParseError when the input is malformed. Both
return Bytes because Base64 can represent arbitrary binary data; call
`.decode()` on the result only when the decoded bytes are known to contain
UTF-8 text.

~~~goblin
var token = base64.url_encode(Bytes([251, 255]))
print(token)
print(base64.url_decode(token))
~~~

Base64 is an encoding, not encryption. Do not use it to conceal passwords,
tokens, or other secrets.
