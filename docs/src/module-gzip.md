# gzip

The gzip module reads and writes gzip data: the `.gz` files that exports and
log rotation produce, and compressed request bodies. `compress` and
`decompress` work on a whole value in memory; `open` and `read_lines` work on a
`.gz` **file** of any size, a piece at a time.

## Common tasks

~~~goblin
import "exec"
import "fs"
import "gzip"
import "http"
import "json"

# A compressed export of any size, one line at a time
var errors = 0
for line in gzip.read_lines("events.jsonl.gz") {
    if json.decode(line)["level"] == "error" {
        errors = errors + 1
    }
}

# A small one, whole
var config = json.decode(gzip.decompress(fs.read_bytes("config.json.gz")))

# Write a compressed file piece by piece
var out = gzip.open("report.csv.gz", write=true)
defer out.close()
for row in rows {
    out.write("{row}\n")
}

# Compress a big log without holding it
var packed = gzip.open("app.log.gz", write=true, level=9)
defer packed.close()
for line in fs.read_lines("app.log") {
    packed.write("{line}\n")
}

# Compress a small file in one line
fs.write("notes.txt.gz", gzip.compress(fs.read_bytes("notes.txt")))

# Unpack to disk without holding it
var source = gzip.open("dump.sql.gz")
defer source.close()
fs.write("dump.sql", source)

# Compress a command's output as it is produced
var dump = gzip.open("shop.sql.gz", write=true)
defer dump.close()
exec.run(["pg_dump", "shop"], stdout=dump)

# Download a big compressed export, then go through it
fs.write("export.jsonl.gz", http.get(export_url).body)
for line in gzip.read_lines("export.jsonl.gz") {
    print(json.decode(line)["id"])
}

# Compress before uploading
var body = gzip.compress(json.encode(records))
http.request("POST", upload_url, headers={"Content-Encoding": "gzip", "Content-Type": "application/json"}, body=body)
~~~

`examples/gzip.goblin` runs the tasks that need no network or database.

## Module members

| Member | Returns | Behaviour |
| --- | --- | --- |
| `compress(data, level=nil)` | Bytes | `data` (a String's UTF-8 bytes, Bytes, or a readable read to its end) in gzip format. `level` is 1 (fastest) to 9 (smallest); `nil` is the usual balance |
| `decompress(data)` | Bytes | The original bytes. `data` is Bytes or a readable such as an open File; a String is a TypeError, since gzip data is binary. Several gzip members one after another are decompressed as one |
| `read_lines(path)` | `fs.Lines` | The twin of [`fs.read_lines`](./module-fs.md#reading-lines): each `for` over it opens the file, yields the decompressed text one line per step (terminator removed) and closes the file when the loop ends by any route. It never holds more than one line |
| `open(path, write=false, level=nil)` | `fs.File` | The twin of `fs.open`. Read-only by default: the file must exist and be gzip, and the File reads its **decompressed** content. `write=true`: the file is created or truncated, and everything written is compressed into it at `level` |

`decompress` returns Bytes; `.decode()` it when the content is text.

## Streams

`gzip.open` returns an ordinary [File](./module-fs.md#file): no new type,
nothing new to learn. `type_of(f) == fs.File`, it shows as
`<File events.json.gz>` and `f.path` is the path given. Code written for a
plain file works on a `.gz` by changing `fs.` to `gzip.`.

| File member | Opened for reading | Opened with `write=true` |
| --- | --- | --- |
| `read(size=nil)` | Decompressed Bytes; empty at the end | ValueError, as any File open for writing |
| `read_line()`, `for line in f` | Decompressed lines as Strings | ValueError |
| `write(data)` | ValueError, as any File open for reading | Compresses the String or Bytes into the file; returns the number of bytes given, not the compressed size |
| `close()` | Releases the file | **Finishes the gzip data**, then releases the file |
| `closed`, `path` | as on any File | as on any File |

Until a writing File is closed the file on disk is incomplete, so a writer
always gets `defer f.close()`. As with any File, `for line in f` does not
close `f`: whoever opened it closes it.

A reading File is a readable, so it can be passed as the `data` of
`csv.decode_dicts(f)`, `json.decode(f)`, `hash.sha256(f)` or
`fs.write(path, f)`. A writing File is a writable, so it can be the `stdout=`
of `exec.run`.

A response sent with `Content-Encoding: gzip` is already decompressed by
[http](./module-http.md)'s `get`, `request` and `send`; a downloaded `.gz`
**file** goes to disk first with `fs.write(path, http.get(url).body)`, then
through `gzip.read_lines(path)`.

### Errors

| Failure | Raises |
| --- | --- |
| `level` outside 1 to 9 | ValueError `compress() argument 'level' must be from 1 to 9, got 0` |
| `level` given to `open` without `write=true` | ValueError |
| `data` does not start like gzip (empty data included) | ParseError `decompress() not gzip data` |
| The data is truncated or damaged | ParseError `decompress() corrupt gzip data` |
| `data` has the wrong type | TypeError |
| Reading a readable `data` fails | the error the readable raised, such as IOError |
| `open` or a `read_lines` loop on a missing or unreadable file | NotExistError, PermissionError, IOError, as `fs` |
| The file is not gzip | ParseError `open() "x.gz" is not gzip data`, `read_lines() "x.gz" is not gzip data` (when the loop starts) |
| The file is damaged further in | ParseError from the call that reaches the damage: `read() corrupt gzip data in "x.gz"`, `read_lines() corrupt gzip data in "x.gz"` |
| A line is not valid UTF-8 | ValueError `line 3 of "x.gz" is not valid UTF-8` |

Every error of `open` and `read_lines`, and of the File they give, that is
about the file carries `e.path`, a Path; the invalid-UTF-8 ValueError also
carries `e.line`, as in [fs](./module-fs.md#errors).

## Left out

Wrapping a readable or writable that is not a file (`open` takes a path, like
every `open`: decompress a response body or a pipe in memory with
`gzip.decompress(readable)`, or through a file); appending to an existing
`.gz`; gzip header fields such as the stored file name; and every other
format: zlib, raw DEFLATE, bzip2 and LZW. Archives are other modules:
[tar](./module-tar.md) reads a `.tar.gz` by itself and writes one with
`tar.create(path, gzip=true)`, and [zip](./module-zip.md) handles `.zip`.
