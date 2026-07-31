# hmac, crc64, and fnv

These modules wrap Go's `crypto/hmac`, `hash/crc64`, and `hash/fnv`
packages. They accept either str or Bytes input. Digest values are returned as
Bytes by `sum()` and lowercase text by `hex()`.

~~~goblin
import "hmac"
import "crc64"
import "fnv"

var signature = hmac.sum("secret", "message")
print(hmac.verify(signature, "secret", "message"))
print(crc64.hex("123456789"))
print(fnv.hex("hello"))
~~~

## HMAC

| Function | Returns | Description |
| --- | --- | --- |
| `sum(key, data, algorithm="sha256")` | Bytes | Compute an HMAC digest |
| `hex(key, data, algorithm="sha256")` | str | Compute an HMAC as hexadecimal text |
| `verify(signature, key, data, algorithm="sha256")` | bool | Compare a raw signature in constant time |

Supported algorithms are `sha256`, `sha512`, `sha1`, and `md5`. SHA-1 and MD5
are provided only for compatibility with existing protocols. `verify()` expects
the raw Bytes returned by `sum()`, not hexadecimal text.

## CRC-64

`crc64.sum(data, polynomial="ecma")` and `crc64.hex(...)` support the `ecma`
and `iso` polynomials. Unlike `crc32.checksum()`, CRC-64 returns Bytes because a
Goblin int is signed and cannot represent every unsigned 64-bit checksum.

## FNV

`fnv.sum(data, variant="64a")` and `fnv.hex(...)` support `32`, `32a`, `64`,
`64a`, `128`, and `128a`. FNV is a non-cryptographic hash; do not use it for
passwords, signatures, or integrity checks against an attacker.
