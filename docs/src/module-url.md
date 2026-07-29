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
The `reference` argument must itself be a value returned by `url.parse()`.
Malformed input raises `ParseError`.

~~~goblin
import "url"

var endpoint = url.parse("https://example.com:8443/api?q=goblin#result")
print(endpoint.scheme)   # https
print(endpoint.hostname) # example.com
print(endpoint.port)     # 8443
print(endpoint.path)     # /api
print(endpoint.raw_query)

var next = endpoint.resolve_reference(url.parse("../status"))
print(Str(next))
~~~

`join_path(base, elements)` accepts a base URL and a list of path elements. It
normalizes path separators while preserving the URL's scheme and host:

~~~goblin
print(url.join_path("https://example.com/api", ["v1", "items"]))
print(url.query_escape("name=Goblin language"))
print(url.path_escape("folder/item"))
~~~

Query escaping is for a query component; path escaping is for one path
segment. Neither function builds a complete query string from a dictionary.
The corresponding unescape functions raise `ParseError` for invalid percent
escapes.
