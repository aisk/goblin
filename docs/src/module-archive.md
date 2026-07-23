# tar and zip

The `tar` and `zip` modules adapt Go's `archive/tar` and `archive/zip` packages
to complete in-memory archives.

Both modules provide `write_all(files)` and `read_all(data)`. `files` is a
dictionary whose string keys are archive paths and whose values are `Bytes` or
`Str`. `write_all` returns archive `Bytes`; `read_all` returns a dictionary of
file names to `Bytes`. Directory and other non-regular entries are skipped when
reading.

`zip.write_all` accepts `method=zip.deflate` and also supports `zip.store`.
Malformed archives raise `ParseError`.

These whole-archive operations correspond to iterating Go Reader and Writer
entries. A future common Goblin stream protocol can add incremental access
without changing the archive format or these convenience operations.
