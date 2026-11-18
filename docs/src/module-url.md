# url

The url module takes a URL apart, escapes its parts and changes its query. A
`URL` is a value, like [`fs.Path`](./module-fs.md#path): it never touches the
network. Functions that act on what a URL names, such as the
[http](./module-http.md) requests, accept a String or a URL.

## Common tasks

~~~goblin
import "fs"
import "url"

# Take one apart
var link = url.parse("https://example.com:8443/docs/intro?lang=en&page=2#install")
print(link.host, link.port, link.path)        # example.com 8443 /docs/intro
var params = url.decode_query(link.query)
print(params.get("page", default="1"))        # 2

# Every value of a name that repeats
print(url.query_values("tag=a&tag=b&x=1", "tag")) # ["a", "b"]

# The credentials in a connection URL, decoded
var db = url.parse("postgres://ada:p%40ss@db.example.com:5432/shop")
print(db.user, db.password, db.host)          # ada p@ss db.example.com

# Resolve the links found on a page
var page = url.parse("https://example.com/docs/intro")
print(page.join("../img/logo.png"))           # https://example.com/img/logo.png

# Build one
var search = "https://example.com/search?" + url.encode_query({"q": "goblin lang", "tag": ["a", "b"]})
var file_url = "https://example.com/files/{url.path_escape(name)}"

# The same URL with a query parameter changed: the next page
var listing = url.parse("https://example.com/items?sort=price&page=2")
var query = url.decode_query(listing.query)
query["page"] = Int(query["page"]) + 1
print(listing.with_query(query))              # https://example.com/items?sort=price&page=3

# A link without its tracking parameters
func clean(link) {
    var kept = {}
    for name, value in url.decode_query(link.query).items() {
        if !name.has_prefix("utm_") {
            kept[name] = value
        }
    }
    return link.with_query(kept)
}

# The file name a URL ends in
var download = url.parse("https://example.com/files/report%202026.pdf?dl=1")
print(fs.Path(download.path).name)            # report 2026.pdf

# Is it a link we follow
func is_web(text) {
    try {
        var scheme = url.parse(text).scheme
        return scheme == "http" || scheme == "https"
    } catch e: ParseError {
        return false
    }
}

# Report the bad ones
for line in fs.read_lines("links.txt") {
    try {
        print(url.parse(line).host)
    } catch e: ParseError {
        eprint("not a URL: {e.text}")
    }
}
~~~

`docs/examples/url-tasks.goblin` runs these tasks; `examples/url.goblin` covers
the rest of the module.

## Module members

| Member | Returns | Behaviour |
| --- | --- | --- |
| `parse(text)` | URL | The URL `text` spells. Relative references (`"../a?x=1"`, `"/path"`) are valid and have an empty `scheme` and `host` |
| `encode_query(params)` | String | A query string without the leading `?`, from a Dict in its order: `{"q": "a b", "page": 2}` gives `q=a+b&page=2`. Values are String, Int, Float or Bool; a List value repeats the name once per element; a `nil` value leaves the name out. `{}` gives `""` |
| `decode_query(text)` | Dict of String to String | The names and values of a query string, in order; a leading `?` is ignored. A name that repeats keeps its last value. `""` gives `{}` |
| `query_values(text, name)` | List of String | Every value `name` has in the query string `text`, in order; `[]` when it has none. A leading `?` is ignored. It is a module function, not a URL method, because a query string also arrives on its own, as a form body |
| `query_escape(text)` | String | `text` made safe as one query name or value (a space becomes `+`) |
| `path_escape(text)` | String | `text` made safe as one path segment (a space becomes `%20`, `/` becomes `%2F`) |

The type name `URL` is exported for `type_of(u) == url.URL`; calling it raises
`TypeError: URL cannot be constructed; use url.parse(text)`.

## URL

Immutable. Two URLs are equal when their text is (a URL never equals a
String), a URL can be a Dict key, and it prints as the full URL. Inside a List,
a Dict or a structural Show it shows as `URL("https://example.com/")`, the
text quoted like a String element, so it is not mistaken for a String. URLs
have no order.

| Member | Kind | Value | When absent |
| --- | --- | --- | --- |
| `scheme` | property | String, lower case (`"https"`) | `""` |
| `user` | property | String, the user name, decoded (`"ada"` in `postgres://ada:p%40ss@db/shop`) | `""` |
| `password` | property | String, the password, decoded (`"p@ss"`) | `""` |
| `host` | property | String, the host name or address without the port and without IPv6 brackets | `""` |
| `port` | property | Int, 0 to 65535 (`parse` refuses a larger one) | `nil`; it is not filled in from the scheme |
| `path` | property | String, decoded (`"/a b/c"`) | `""` |
| `query` | property | String, as written, without `?` (`"q=a+b&page=2"`); read it with `url.decode_query` | `""` |
| `fragment` | property | String, decoded, without `#` | `""` |
| `join(reference)` | method | The URL that `reference` (a String or URL) means when found on this page, by the rules of RFC 3986. An absolute reference is returned as it is | |
| `with_query(params)` | method | The same URL with its query replaced by `url.encode_query(params)`; `{}` removes the query. Everything else, the fragment included, is kept | |

## Repeated names

Writing and reading are deliberately not mirror images, here and for HTTP
headers:

| | Writing | Reading |
| --- | --- | --- |
| Query (`url`) | `encode_query`, `with_query`: a List value repeats the name | `decode_query`: one String per name; a repeated name keeps its **last** value. `query_values(text, name)` for each |
| Headers ([http](./module-http.md)) | `headers=`: a List value repeats the header | An `http.Headers`: `headers[name]` and `get(name)` give one String per name, repeats **joined** with `", "`; `get_all(name)` for each |

Reading returns one shape whatever the data: a value is never "a String, or a
List when the name repeats". The two differ in what they do with repeats
because the formats do: HTTP defines a repeated header as a comma-joined list,
a query string defines nothing.

## Errors

| Failure | Raises | Data |
| --- | --- | --- |
| `parse(text)` of text that is not a URL | ParseError `parse() invalid URL "ht!tp://x": first path segment in URL cannot contain colon` | `text` |
| `parse(text)` with a port above 65535 | ParseError `parse() invalid URL "http://h:99999/": port out of range` | `text` |
| `join(reference)` with a String that is not a URL | ParseError `join() invalid URL "…": …` | `text`, the reference |
| `decode_query` or `query_values` with a bad `%` escape anywhere in the query | ParseError `decode_query() invalid escape "%zz"` | `text`, the whole query |
| A Dict key that is not a String, a value that is not String, Int, Float, Bool, List or `nil` | TypeError | — |

`e.text` is the String that could not be parsed, whole, so a handler can
report it without keeping the input around.

## Left out

Changing parts of a URL other than the query: build the text with
interpolation, escaping the decoded path again segment by segment, since
`u.path` is decoded text:

~~~goblin
var p = u.path.split("/").map(url.path_escape).join("/")
var moved = "{u.scheme}://{mirror}{p}"
if u.query != "" {
    moved = "{moved}?{u.query}"
}
~~~

Also left out: unescaping functions (`URL.path`, `URL.fragment`, `user`,
`password` and `decode_query` already return decoded text). A URL has one text
form, `String(u)`, so there is no `format`.
