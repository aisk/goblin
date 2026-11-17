# hash

The hash module computes digests of data: a checksum of a download, the
signature of an API request, a content-addressed cache key. Every function
returns the digest as **Bytes**; `digest.to_hex()` is its usual text form.

`import "hash"` binds the name `hash`, which hides the built-in `hash(x)` in
that file. The two are unrelated: the built-in is the Dict hash of a value and
changes between runs, while a digest is the same everywhere and forever.

## Common tasks

~~~goblin
import "hash"
import "fs"

# Checksum of a string
print(hash.sha256("hello").to_hex())

# Verify a download without loading it into memory
var file = fs.open("goblin.tar.gz")
defer file.close()
var actual = hash.sha256(file).to_hex()
if actual != expected {
    raise Error("checksum mismatch: {actual}")
}

# Sign a request
var signature = hash.hmac(secret, body).to_hex()
var headers = {"X-Signature": signature}

# An older scheme that still uses SHA-1
var legacy = hash.hmac(secret, body, algorithm=hash.sha1)
~~~

Pass the headers with the request as the [http](./module-http.md) chapter
shows. `examples/hash.goblin` runs each of these tasks.

## Module members

`data` is a String (its UTF-8 bytes), Bytes, or a readable (anything with a
`read(size)` method, such as an open File), which is read to its end in
chunks and not closed, so a file of any size can be hashed. Empty data is
valid and has a digest.

| Member | Returns | Behaviour |
| --- | --- | --- |
| `sha256(data)` | Bytes (32) | SHA-256 digest. The default choice |
| `sha512(data)` | Bytes (64) | SHA-512 digest |
| `sha1(data)` | Bytes (20) | SHA-1 digest. For compatibility with existing checksums only |
| `md5(data)` | Bytes (16) | MD5 digest. For compatibility with existing checksums only |
| `hmac(key, data, algorithm=hash.sha256)` | Bytes | Keyed digest (HMAC). `key` is a String or Bytes. `algorithm` is one of the four functions above, passed as a value |

### Errors

| Failure | Raises |
| --- | --- |
| `data` is not String, Bytes or a readable | TypeError `sha256() argument 'data' must be String, Bytes or have a read(size) method, got Int` |
| `key` is not String or Bytes | TypeError `hmac() argument 'key' must be String or Bytes, got Int` |
| `algorithm` is anything else, a name string included | ValueError `hmac() argument 'algorithm' must be hash.md5, hash.sha1, hash.sha256 or hash.sha512` |
| Reading the readable fails | the error the readable raised, such as IOError |

## Left out

Incremental hashers (a readable covers the large-input case), SHA-224, SHA-384
and the truncated SHA-512 variants, CRC32, CRC64, Adler-32 and FNV checksums,
constant-time comparison (Goblin has no server side to receive signatures:
compare with `==`), and password hashing.
