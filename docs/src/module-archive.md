# tar and zip

The `tar` and `zip` modules adapt Go's `archive/tar` and `archive/zip` packages
to complete in-memory archives.

Both modules provide `write_all(files)` and `read_all(data)`. `files` is a
dictionary whose string keys are archive paths and whose values are `Bytes` or
`Str`. `write_all` returns archive `Bytes`; `read_all` returns a dictionary of
file names to `Bytes`. Directory and other non-regular entries are skipped when
reading.

`zip.write_all` accepts `method=zip.DEFLATE` and also supports `zip.STORE`.
Malformed archives raise `ParseError`.

~~~goblin
import "zip"

var archive = zip.write_all({
    "README.txt": "Goblin archive",
    "data/raw.bin": Bytes("abc"),
})
var files = zip.read_all(archive)
print(files["README.txt"].decode())
~~~

Use `method=zip.STORE` when entries are already compressed or must be stored
verbatim; the default `zip.DEFLATE` generally produces smaller archives.
`tar.write_all(files)` has the same dictionary input but no compression-method
argument.

Archive paths are taken from the dictionary keys. Validate untrusted names
before writing returned entries to disk: `read_all()` keeps archive names and
does not choose a safe extraction directory for the application. Duplicate
entry names collapse to one dictionary key when reading.

These whole-archive operations correspond to iterating Go Reader and Writer
entries. A future common Goblin stream protocol can add incremental access
without changing the archive format or these convenience operations.
