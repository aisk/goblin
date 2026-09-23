# Standard library

Goblin's standard library provides modules for common program boundaries:
files, environment variables, external commands, paths, time, data formats, networking, random
values, mathematics, and MIME metadata. Import a module at module scope, then
access its members with dot notation.

~~~goblin
import "json"
import "path"

var cwd = path.cwd()
var text = json.marshal({"cwd": cwd})
print(text)
~~~

The standard library is separate from Goblin's built-in functions and types.
For example, print(), eprint(), range(), Int(), List(), Dict(), and Chan() are
available without an import. Import a module only when its capabilities are
needed.

## Two tiers: core and x/

The standard library has two tiers. Core modules have curated, Goblin-shaped
APIs and simple names such as "json" and "fs". Modules under the `x/` prefix
are direct adaptations of Go packages and keep Go's package hierarchy in their
import path, so `compress/gzip` becomes `x/compress/gzip`. In both tiers the
imported name is the last path component:

~~~goblin
import "x/compress/gzip"

var packed = gzip.compress("hello")
~~~

## Core modules

| Module | Main purpose | Start with |
| --- | --- | --- |
| [json](./module-json.md) | Encode and decode JSON | marshal(), unmarshal() |
| [fs](./module-fs.md) | Read, write, inspect, and remove files | read(), write(), exists() |
| [os](./module-os.md) | Read environment and process information | argv(), getenv(), getwd(), hostname() |
| [exec](./module-exec.md) | Configure and execute external commands | Command() |
| [path](./module-path.md) | Find the current or home directory | cwd(), home() |
| [time](./module-time.md) | Work with time and durations | now(), sleep(), parse() |
| [rand](./module-rand.md) | Generate reproducible random values and permutations | Rand(), int(), shuffle() |
| [math](./module-math.md) | Numeric constants and functions | pi, sqrt(), pow(), abs() |
| [http](./module-http.md) | Make HTTP requests | get(), post(), put() |
| [uuid](./module-uuid.md) | Generate and parse UUID values | UUID(), new(), NIL |
| [regexp](./module-regexp.md) | Search, capture, replace, and split text with RE2 expressions | compile(), escape() |
| [url](./module-url.md) | Parse, resolve, join, and escape URLs | parse(), query_escape() |
| [csv](./module-csv.md) | Read and write comma-separated records | read_all(), write_all() |
| [io](./module-io.md) | Implement readers and writers as traits | Reader, Writer |

## x/ modules

| Module | Main purpose | Start with |
| --- | --- | --- |
| [x/encoding/base64](./module-base64.md) | Encode and decode Base64 text | encode(), decode() |
| [x/encoding/base32, ascii85, html, quotedprintable](./module-text-encoding.md) | Encode text and escape HTML | encode(), escape() |
| [x/encoding/hex](./module-hex.md) | Encode, decode, and dump hexadecimal data | encode(), decode() |
| [x/encoding/pem](./module-pem.md) | Encode and decode PEM blocks | Block(), decode() |
| [x/mime](./module-mime.md) | Look up MIME types and extensions | type_by_extension() |
| [x/crypto/sha256 and sha512](./module-sha2.md) | Compute fixed-size SHA-2 digests | sum(), hex() |
| [x/crypto/md5, sha1, x/hash/crc32, adler32](./module-checksum.md) | Compute compatibility digests and checksums | hex(), checksum() |
| [x/crypto/hmac, x/hash/crc64, fnv](./module-crypto-hash.md) | Compute keyed and non-cryptographic hashes | sum(), hex() |
| [x/compress/gzip, zlib, flate, bzip2](./module-compression.md) | Compress and decompress complete byte values | compress(), decompress() |
| [x/compress/lzw](./module-lzw.md) | Compress and decompress LZW data | compress(), decompress() |
| [x/archive/tar and zip](./module-archive.md) | Read and write complete in-memory archives | read_all(), write_all() |
| [x/net/mail](./module-mail.md) | Construct and parse email addresses | parse_address() |
| [x/net/netip](./module-netip.md) | Parse and calculate with IP addresses and prefixes | Addr(), Prefix() |
| [x/unicode and x/unicode/utf8](./module-unicode.md) | Validate UTF-8 and classify Unicode characters | valid(), is_letter() |

## Imports and errors

Standard-library module names never start with "./" or "../"; core names are
plain ("json", "fs") and x/ names carry their Go-style path
("x/compress/gzip"). Local source modules use a relative import such as
"./modules/greeter"; those are documented in
[Modules and imports](./modules.md) because they use the same import syntax.

Most standard-library operations that touch the outside world can fail. JSON
parsing may raise ParseError, a missing file may raise an I/O-related error, and
HTTP requests may fail. Use try/catch around work that your program can recover
from; see [Errors](./errors.md).

## Choosing a module

Use json whenever a program boundary expects JSON rather than trying to build
JSON text manually. Prefer fs for simple whole-file reads and writes, and use
its open() function when a file object is needed. Use path.cwd() or path.home()
instead of assuming a current directory. Use time.sleep() only for intentional
delays, and use Chan plus spawn() for communication between concurrent Goblin
functions.

Each module has its own chapter in this section, with a focused API reference
and example.

## Reading API signatures

Examples and tables use `name(required, optional=value)` to show argument
order and defaults. Square brackets mean an argument may be omitted, as in
`Chan([size])`. They do not promise that named arguments are accepted: a
function's chapter calls out positional-only APIs where that matters.

Unless a chapter says otherwise, a function that touches files, the operating
system, or the network can raise an error value. Wrap the smallest useful
boundary in try/catch, then add context or recover deliberately.
