# md5, sha1, crc32, and adler32

These modules calculate complete, in-memory digests and checksums. Every
function accepts either `Bytes` or `Str`.

## MD5 and SHA-1

`md5.sum(data)` and `sha1.sum(data)` return raw digest `Bytes`.
`md5.hex(data)` and `sha1.hex(data)` return lowercase hexadecimal text.

MD5 and SHA-1 are provided for compatibility with existing formats and
protocols. They are cryptographically broken and must not be used for password
storage, signatures, certificates, or other security decisions.

~~~goblin
import "md5"
import "sha1"

print(md5.hex("Goblin"))
print(sha1.sum(Bytes("Goblin")).size()) # 20 bytes
~~~

## CRC-32

`crc32.checksum(data, polynomial=crc32.IEEE)` returns an unsigned 32-bit
checksum as a Goblin `int`. The module exports the `IEEE`, `CASTAGNOLI`, and
`KOOPMAN` polynomial constants.

## Adler-32

`adler32.checksum(data)` returns the Adler-32 checksum as a Goblin `int`.

CRC-32 and Adler-32 detect accidental corruption; they do not authenticate
data and are not cryptographic hashes.

~~~goblin
import "crc32"
import "adler32"

print(crc32.checksum("Goblin"))
print(crc32.checksum("Goblin", polynomial=crc32.CASTAGNOLI))
print(adler32.checksum("Goblin"))
~~~

The returned checksum integers are non-negative. Choose the CRC polynomial as
part of the surrounding file or wire-format contract; values calculated with
different polynomials are not comparable.
