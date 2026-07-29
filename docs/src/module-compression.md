# gzip, zlib, flate, and bzip2

The `gzip`, `zlib`, and `flate` modules adapt Go's corresponding `compress`
packages to whole Goblin values.

All three modules expose `compress(data, level=DEFAULT_COMPRESSION)` and
`decompress(data)`. Input may be `Bytes` or `Str`; output is always `Bytes`.
Malformed compressed input raises `ParseError`.

Compression-level constants mirror `compress/flate`:
`NO_COMPRESSION`, `BEST_SPEED`, `BEST_COMPRESSION`, `DEFAULT_COMPRESSION`, and
`HUFFMAN_ONLY`.

These whole-value helpers correspond to creating a Go Writer or Reader,
processing the complete value, and closing it. They avoid exposing a separate
stream abstraction before Goblin has a common Reader/Writer protocol.

The `bzip2` module exposes only `decompress(data)`, returning `Bytes`. This
mirrors Go's `compress/bzip2`, which provides a reader but no compressor.
