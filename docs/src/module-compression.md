# gzip and zlib

The `gzip` and `zlib` modules adapt Go's `compress/gzip` and `compress/zlib`
packages to whole Goblin values.

Both modules expose `compress(data, level=DEFAULT_COMPRESSION)` and
`decompress(data)`. Input may be `Bytes` or `Str`; output is always `Bytes`.
Malformed compressed input raises `ParseError`.

Compression-level constants mirror `compress/flate`:
`NO_COMPRESSION`, `BEST_SPEED`, `BEST_COMPRESSION`, `DEFAULT_COMPRESSION`, and
`HUFFMAN_ONLY`.

These whole-value helpers correspond to creating a Go Writer or Reader,
processing the complete value, and closing it. They avoid exposing a separate
stream abstraction before Goblin has a common Reader/Writer protocol.
