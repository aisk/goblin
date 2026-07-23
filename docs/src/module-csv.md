# csv

The `csv` module follows Go's `encoding/csv` package and exposes its whole-data
operations.

| Function | Go equivalent |
| --- | --- |
| `read_all(text, ...)` | `Reader.ReadAll` |
| `write_all(records, ...)` | `Writer.WriteAll` |

`read_all` returns a list of string lists. Keyword arguments configure the
corresponding Go Reader fields: `comma=","`, `comment=""`,
`fields_per_record=0`, `lazy_quotes=false`, and
`trim_leading_space=false`.

`write_all` accepts a list of string lists. It supports `comma=","` and
`use_crlf=false`, corresponding to Go's Writer fields. Parsing errors raise
`ParseError`.
