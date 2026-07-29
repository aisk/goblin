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

## Module map

| Module | Main purpose | Start with |
| --- | --- | --- |
| [json](./module-json.md) | Encode and decode JSON | marshal(), unmarshal() |
| [base64](./module-base64.md) | Encode and decode Base64 text | encode(), decode() |
| [base32, ascii85, html, and quotedprintable](./module-text-encoding.md) | Encode text and escape HTML | encode(), escape() |
| [fs](./module-fs.md) | Read, write, inspect, and remove files | read(), write(), exists() |
| [os](./module-os.md) | Read environment and process information | argv(), getenv(), getwd(), hostname() |
| [exec](./module-exec.md) | Configure and execute external commands | Command() |
| [path](./module-path.md) | Find the current or home directory | cwd(), home() |
| [time](./module-time.md) | Work with time and durations | now(), sleep(), parse() |
| [rand](./module-rand.md) | Generate reproducible random values and permutations | Rand(), int(), shuffle() |
| [math](./module-math.md) | Numeric constants and functions | pi, sqrt(), pow(), abs() |
| [http](./module-http.md) | Make HTTP requests | get(), post(), put() |
| [mime](./module-mime.md) | Look up MIME types and extensions | type_by_extension() |
| [uuid](./module-uuid.md) | Construct, generate, and validate UUID values | UUID(), new(), validate() |
| [regexp](./module-regexp.md) | Search, capture, replace, and split text with RE2 expressions | compile(), escape() |
| [hex](./module-hex.md) | Encode, decode, and dump hexadecimal data | encode(), decode() |
| [sha256 and sha512](./module-sha2.md) | Compute fixed-size SHA-2 digests | sum(), hex() |
| [md5, sha1, crc32, and adler32](./module-checksum.md) | Compute compatibility digests and checksums | hex(), checksum() |
| [url](./module-url.md) | Parse, resolve, join, and escape URLs | parse(), query_escape() |
| [csv](./module-csv.md) | Read and write comma-separated records | read_all(), write_all() |
| [gzip, zlib, flate, and bzip2](./module-compression.md) | Compress and decompress complete byte values | compress(), decompress() |
| [tar and zip](./module-archive.md) | Read and write complete in-memory archives | read_all(), write_all() |
| [mail](./module-mail.md) | Construct and parse email addresses | parse_address() |

## Imports and errors

Built-in module names are simple strings such as "json" and "fs". Local source
modules use a relative import such as "./modules/greeter"; those are documented
in [Modules and imports](./modules.md) because they use the same import syntax.

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
