# gzip

The gzip module compresses and decompresses whole values in gzip format: the
`.gz` files that exports and log rotation produce, and compressed request
bodies. Both functions work on the complete value in memory and return Bytes.

## Common tasks

~~~goblin
import "gzip"
import "fs"
import "json"

# Read a compressed export
var text = gzip.decompress(fs.read_bytes("events.json.gz")).decode()
for line in text.split("\n") {
    handle(line)
}

# Compress before uploading
var body = gzip.compress(json.encode(records))
var headers = {"Content-Encoding": "gzip", "Content-Type": "application/json"}

# Compress a file
fs.write("app.log.gz", gzip.compress(fs.read_bytes("app.log"), level=9))
~~~

Send `body` with `headers` as the [http](./module-http.md) chapter shows.
`examples/gzip.goblin` runs each of these tasks.

## Module members

| Member | Returns | Behaviour |
| --- | --- | --- |
| `compress(data, level=nil)` | Bytes | `data` (a String's UTF-8 bytes, Bytes, or a readable read to its end) in gzip format. `level` is 1 (fastest) to 9 (smallest); `nil` is the usual balance |
| `decompress(data)` | Bytes | The original bytes. `data` is Bytes or a readable such as an open File; a String is a TypeError, since gzip data is binary. Several gzip members one after another are decompressed as one |

`decompress` returns Bytes; `.decode()` it when the content is text.

### Errors

| Failure | Raises |
| --- | --- |
| `level` outside 1 to 9 | ValueError `compress() argument 'level' must be from 1 to 9, got 0` |
| `data` does not start like gzip (empty data included) | ParseError `decompress() not gzip data` |
| The data is truncated or damaged | ParseError `decompress() corrupt gzip data` |
| `data` has the wrong type | TypeError |
| Reading a readable `data` fails | the error the readable raised, such as IOError |

## Left out

Streaming compression and decompression (a readable input is read whole), gzip
header fields such as the stored file name, and every other format: zlib, raw
DEFLATE, bzip2 and LZW. For zip and tar archives run the system tool, for
example `exec.run(["tar", "-xzf", archive, "-C", dir])`.
