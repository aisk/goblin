# url

The `url` module follows Go's `net/url` parsing and escaping behavior.

| Function | Go equivalent |
| --- | --- |
| `parse(raw_url)` | `url.Parse` |
| `join_path(base, elements)` | `url.JoinPath` |
| `query_escape(s)` / `query_unescape(s)` | `url.QueryEscape` / `url.QueryUnescape` |
| `path_escape(s)` / `path_unescape(s)` | `url.PathEscape` / `url.PathUnescape` |

`parse` returns a URL with `scheme`, `host`, `path`, `raw_query`, `fragment`,
`hostname`, `port`, and `escaped_path` attributes. Its
`resolve_reference(reference)` method mirrors Go's `URL.ResolveReference`.
Malformed input raises `ParseError`.
