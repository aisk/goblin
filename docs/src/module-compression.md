# gzip and zlib

The `gzip` and `zlib` modules adapt Go's `compress/gzip` and `compress/zlib`
packages to whole Goblin values.

Both modules expose `compress(data, level=default_compression)` and
`decompress(data)`. Input may be `Bytes` or `Str`; output is always `Bytes`.
Malformed compressed input raises `ParseError`.

Compression-level constants mirror `compress/flate`:
`no_compression`, `best_speed`, `best_compression`, `default_compression`, and
`huffman_only`.

These whole-value helpers correspond to creating a Go Writer or Reader,
processing the complete value, and closing it. They avoid exposing a separate
stream abstraction before Goblin has a common Reader/Writer protocol.
