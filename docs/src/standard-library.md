# Standard library

Goblin's standard library provides modules for common program boundaries:
files and paths, environment variables, external commands, time, data formats,
digests and compression, networking, random values, and mathematics. Import a module at module scope, then
access its members with dot notation.

~~~goblin
import "json"
import "fs"

var cwd = fs.cwd()
var text = json.encode({"cwd": String(cwd)})
print(text)
~~~

The standard library is separate from Goblin's built-in functions and types.
For example, print(), eprint(), range(), Int(), List(), Dict(), and Chan() are
available without an import. Import a module only when its capabilities are
needed.

## Modules

| Module | Main purpose | Start with |
| --- | --- | --- |
| [json](./module-json.md) | JSON text to values and back | encode(), decode() |
| [fs](./module-fs.md) | Files, directories and path names | read(), write(), Path() |
| [os](./module-os.md) | Arguments, environment, standard streams, exit | args, env, stdin, exit() |
| [cli](./module-cli.md) | Command-line arguments, options, help and usage errors | parse(), Option(), Flag(), Command() |
| [exec](./module-exec.md) | Run other programs | run(), capture(), start() |
| [time](./module-time.md) | Instants, the clock, time zones, sleeping | now(), parse(), Time(), sleep() |
| [rand](./module-rand.md) | Random choices, secure bytes and ids, reproducible on request | choice(), int(), bytes(), uuid(), Rand() |
| [math](./module-math.md) | Float functions, constants and basic statistics | PI, sqrt(), round(), mean() |
| [http](./module-http.md) | HTTP client: requests, sessions, uploads, streamed bodies | get(), post(), Client(), multipart() |
| [regex](./module-regex.md) | Search and rewrite text by pattern | find(), find_all(), replace(), split() |
| [url](./module-url.md) | Take a URL apart, escape its parts, change its query | parse(), encode_query(), decode_query() |
| [csv](./module-csv.md) | CSV text to rows and back | decode(), decode_dicts(), encode() |
| [hash](./module-hash.md) | Digests of data: checksums and HMAC signatures | sha256(), hmac() |
| [base64](./module-base64.md) | Bytes as Base64 text and back | encode(), decode() |
| [gzip](./module-gzip.md) | Gzip data, in memory and as files of any size | compress(), decompress(), open(), read_lines() |

## Imports and errors

Standard-library module names are single words ("json", "fs") and never start
with "./" or "../". Local source modules use a relative import such as
"./modules/greeter"; those are documented in
[Modules and imports](./modules.md) because they use the same import syntax.

Most standard-library operations that touch the outside world can fail. JSON
parsing may raise ParseError, a missing file may raise an I/O-related error, and
HTTP requests may fail. Use try/catch around work that your program can recover
from; see [Errors](./errors.md).

## Choosing a module

Use json whenever a program boundary expects JSON rather than trying to build
JSON text manually. Prefer fs for simple whole-file reads and writes, and use
its open() function when a file object is needed. Use fs.cwd() or fs.home()
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
