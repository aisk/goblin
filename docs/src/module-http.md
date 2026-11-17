# http

The http module is an HTTP client. A request is one call, keywords say what
it carries, and the `Response` is the body as a stream. Anything that is a
policy (which base a path is relative to, when to try again, how a file
becomes a form) is written by you, with the pieces below.

## Common tasks

~~~goblin
import "base64"
import "fs"
import "http"
import "json"
import "time"

# Fetch JSON. A 404 or 500 raises, so the next line can trust what it got
var repo = http.get("https://api.github.com/repos/aisk/goblin").json()
print(repo["stargazers_count"])

# Query parameters
var found = http.get("https://api.example.com/search", params={"q": "goblin", "page": 2})
for item in found.json()["items"] {
    print(item["name"])
}

# Send JSON, send a form
var created = http.post("https://api.example.com/users", json={"name": "Ada"})
print(created.status, created.headers.get("location"))
http.post("https://example.com/contact", form={"email": "ada@example.com", "message": "hello"})

# Upload a file with two form fields
var photo = fs.open("photo.png")
defer photo.close()
var body, content_type = http.multipart(
    fields={"title": "Me", "album": "2026"},
    files={"avatar": photo},
)
http.post("https://example.com/profile", body=body, headers={"Content-Type": content_type})

# Upload something you made, as a file, saying what it is
var report = {"name": "report.csv", "data": csv_text, "content_type": "text/csv"}
var report_body, report_type = http.multipart(files={"report": report})
http.post("https://example.com/reports", body=report_body, headers={"Content-Type": report_type})

# Send a big file as the body, without holding it in memory
var backup = fs.open("backup.tar.gz")
defer backup.close()
http.put("https://storage.example.com/backup.tar.gz", body=backup, timeout=nil)

# Download to a file without holding it in memory; an error page is never saved
fs.write("goblin.tar.gz", http.get(release_url, timeout=nil))

# Where did a redirect end
var landed = http.get("https://example.com/latest")
print(landed.url, landed.url.path)

# Try again when the network or the server has a bad moment
func get_patiently(target, attempts=4) {
    var pause = 1
    for attempt in range(attempts - 1) {
        try {
            return http.get(target, timeout=10)
        } catch e: http.StatusError {
            if ![429, 502, 503, 504].contains(e.status) {
                raise e
            }
            var advice = e.response.headers.get("retry-after")
            if advice != nil {
                pause = Int(advice)     # seconds; a server that sends a date needs time.parse
            }
        } catch e: NetworkError {
        } catch e: TimeoutError {
        }
        time.sleep(pause)
        pause = pause * 2
    }
    return http.get(target, timeout=10)     # the last attempt raises whatever it meets
}
var prices = get_patiently("https://api.example.com/prices").json()

# Handle a status you expect
try {
    http.delete("https://api.example.com/users/7")
} catch e: http.StatusError {
    if e.status != 404 {
        eprint(e.response.text())
        raise e
    }
}

# A status that is an answer, not a failure: wait until a job is there
var job = http.get(job_url, check=false)
while job.status == 404 {
    time.sleep(2)
    job = http.get(job_url, check=false)
}
print(job.json()["state"])

# An API you call many times: shared headers in a Client, the address in a variable
var api = "https://api.example.com/v1"
var client = http.Client(headers={"Authorization": "Bearer {token}"}, timeout=10)
var id = client.get("{api}/me").json()["id"]
client.patch("{api}/users/{id}", json={"active": false})
client.delete("{api}/sessions/current")

# User name and password (Basic authentication)
var credentials = base64.encode("{user}:{password}")
var inbox = http.get("https://example.com/inbox", headers={"Authorization": "Basic {credentials}"})

# Log in, then fetch pages as that user: the Client keeps the session cookie
var shop = "https://example.com"
var site = http.Client()
site.post("{shop}/login", form={"user": "ada", "password": secret})
var orders = site.get("{shop}/account/orders").text()

# A cookie the site wants echoed in a header
var csrf = site.cookies(shop)["csrftoken"]
site.post("{shop}/account/email", form={"email": "ada@example.com"}, headers={"X-CSRFToken": csrf})

# Keep the session for the next run
fs.write("session.json", json.encode(site.cookies(shop)))
var saved = json.decode(fs.read("session.json"))
var resumed = http.Client(cookies={shop: saved})

# An internal service with a self-signed certificate. This gives up knowing who answers
var lab = http.Client(insecure=true)
print(lab.get("https://build.lab.internal/status").json()["state"])

# A stream of events (server-sent events, one JSON document per line): each line as it arrives
var events = http.get("https://api.example.com/events", timeout=nil)
defer events.close()
for line in events {
    if line.has_prefix("data: ") {
        var event = json.decode(line.trim_prefix("data: "))
        print(event["type"])
    }
}

# A request held as a value: a Dict, expanded at the call
var ping = {"url": "https://api.example.com/ping", "json": {"from": "goblin"}, "timeout": 5}
http.post(**ping)

# Every cookie a server set, as it was sent
for cookie in landed.header_values("set-cookie") {
    print(cookie.split(";")[0])
}

# Where a short link points, without following it
var peek = http.Client(redirects=false)
print(peek.get("https://example.com/s/x7").headers.get("location"))
~~~

`examples/httpserver/testdata/http_tasks.goblin` runs each of these tasks
against a local server that the example tests start, through the interpreter
and as a compiled program; `examples/http.goblin` covers what needs no server.

## Module members

| Member | Returns |
| --- | --- |
| `get(url, params=nil, headers=nil, timeout=30, check=true)` | Response |
| `delete(url, params=nil, headers=nil, timeout=30, check=true)` | Response |
| `post(url, body=nil, json=nil, form=nil, params=nil, headers=nil, timeout=30, check=true)` | Response |
| `put(url, …)`, `patch(url, …)` | Response; the keywords of `post` |
| `request(method, url, …)` | Response; the keywords of `post`. `request("HEAD", url)` is a HEAD request |
| `multipart(fields=nil, files=nil)` | List `[body, content_type]`: a `multipart/form-data` body and the header value that goes with it. Sends nothing. See [Multipart bodies](#multipart-bodies) |
| `Client(headers=nil, timeout=30, cookies=nil, insecure=false, redirects=true)` | Client |
| `StatusError` | error kind, see [Errors](#errors) |

`url` is a String or a [`url.URL`](./module-url.md#url), and always absolute:
there is no base to resolve against. A URL built from parts is an
interpolation, `"{api}/users/{id}"`.

The type names `Client` and `Response` are exported for `type_of`; calling
`http.Response()` raises `TypeError: Response cannot be constructed; use
http.get(url)`.

### What a request carries

| Keyword | Accepts | Meaning |
| --- | --- | --- |
| `params=nil` | Dict | Added to the URL's query string, as `url.encode_query` does: a List value repeats the name, `nil` leaves it out |
| `headers=nil` | Dict of String to String or List of String | Request headers. Names are case-insensitive. A List sends the header once per element |
| `body=nil` | String, Bytes, readable | The body as it is. No `Content-Type` is set for you. A readable is **streamed** as it is read, never held whole, so a file of any size can be sent; it is read to its end and not closed. See [How a body is sent](#how-a-body-is-sent) |
| `json=nil` | any value `json.encode` accepts | Sent as JSON with `Content-Type: application/json` |
| `form=nil` | Dict | Form fields, sent as `application/x-www-form-urlencoded` and encoded as `url.encode_query` does |

### How the exchange is run

| Keyword | Accepts | Meaning |
| --- | --- | --- |
| `timeout=30` | seconds or `nil` | Limit for the whole exchange, **sending and reading the body included**. `nil` waits forever: use it for large uploads and downloads, and for streams |
| `check=true` | Bool | A status of 400 or above raises `http.StatusError`. `false` returns the Response for every status |

At most one of `body`, `json` and `form` may be given. A header you pass wins
over one the library would set (`Content-Type` of `json=` and `form=`,
`User-Agent`, `Accept-Encoding`). `Content-Length` and `Transfer-Encoding`
belong to the library and are a ValueError when passed. A `Host` header sets
the host the request names.

**Cookies of a module function.** A call keeps the cookies it is given for the
length of its own redirect chain (a login that answers "302, here is your
cookie" works) and forgets them when it returns. Two module calls never share
cookies; a session is a [Client](#client).

### What a request does without being asked

The complete list. Nothing else is added, repeated or decided by the library.

| Behaviour | Detail | To change it |
| --- | --- | --- |
| Redirects are followed, up to 10 | 301, 302, 303: the next request is a `GET` without a body. 307, 308: same method and body, when the body can be sent again; otherwise the 307 or 308 Response is returned as it is. `Authorization` and `Cookie` headers you passed are not forwarded to another domain | `Client(redirects=false)`. `Response.url` says where it ended |
| gzip is asked for and decoded | `Accept-Encoding: gzip` is sent; a gzip body is decoded as it is read, and `content-encoding` and `content-length` are then absent from `headers` | Pass your own `Accept-Encoding`: the body then arrives as the server sent it |
| The proxy named by the environment is used | `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` | The environment |
| `User-Agent: goblin` | Sent with every request | `headers=` |
| Connections are kept and reused | By module functions among themselves, and by a Client | — |
| TLS certificates are verified | Against the system's store (`SSL_CERT_FILE`/`SSL_CERT_DIR` on Linux) | `Client(insecure=true)` |

## Errors

| Failure | Raises | Data |
| --- | --- | --- |
| The server answered with a status of 400 or above, and `check=true` | `http.StatusError` `get() https://api.example.com/users/9 returned 404 Not Found` | `status` Int; `response` the Response |
| No response arrived (DNS, connection, TLS, reading the body) | NetworkError `get() cannot reach example.com`; the Go error is the `cause` | — |
| `timeout` expired | TimeoutError `get() timed out after 30 seconds` | — |
| `url` is not a URL | ParseError `get() invalid URL "ht!tp://x": …`, as `url.parse` | `text` |
| `url` has no scheme and host (`"users/7"`) or a scheme other than http and https; more than one of `body`, `json`, `form`; `Content-Length` or `Transfer-Encoding` in `headers=`; an invalid `method` | ValueError, before anything is sent | — |
| `json=` cannot be encoded | whatever `json.encode` raises | — |
| A readable given as `body=`, or a part of a multipart body, raises while it is sent | That error, as the readable raised it (a closed File is its ValueError). The request is abandoned | whatever it carries |
| A readable's length changed while it was sent | NetworkError `post() the body changed size while it was sent to example.com` | — |
| `body=` is a multipart body that was already read from | ValueError `post() this multipart body was already sent; build it again`, before anything is sent | — |
| `multipart()` with a value in `files` that is not a part | TypeError for a wrong type; ValueError for a missing `name` or `data`, or an unknown key | — |
| `Response.json()` on a body that is not JSON | ParseError, as `json.decode` | `line`, `column` |
| `for line in response` reaches a line that is not valid UTF-8 | ValueError `line 3 of the response body is not valid UTF-8` | `line` |

About `StatusError`:

- The URL in the message is the one requested, with `params` applied.
- Before it is raised the rest of the body is read into memory (within
  `timeout`) and the connection is released. `e.response` therefore holds no
  resource, and `text()`, `json()`, `read()`, `status`, `headers`,
  `header_values` and `url` work on it as on any Response. That is everything
  a retry loop reads: `e.status`, and `e.response.headers.get("retry-after")`.
- It is neither a NetworkError nor an IOError: the exchange worked and the
  server said no. `catch e: IOError` is "could not talk to the server"
  (NetworkError and TimeoutError are both IOErrors, and siblings: catching one
  does not catch the other); `catch e: http.StatusError` is "the server
  refused".

## Response

The status, the headers, and the body as a readable stream. The connection is
released when the body has been read to its end or `close()` is called, so a
response whose body you consume needs no `defer`. It shows as
`<Response 200 OK>`.

| Member | Kind | Value |
| --- | --- | --- |
| `status` | property | Int (`200`) |
| `status_text` | property | String (`"OK"`, `"Not Found"`) |
| `success` | property | Bool: `status` is below 400, exactly the responses `check=true` returns |
| `url` | property | `url.URL`: where the response came from, which is the requested URL (with `params`) unless a redirect was followed |
| `headers` | property | Dict of String to String. Names are lower case (`"content-type"`). A header sent several times is one entry, its values joined with `", "`. The same Dict on every read |
| `header_values(name)` | method | List of String: every value of that header as it was sent, in order. `name` in any case. `[]` when the header is absent. This is the reading for `set-cookie`, whose values contain commas |
| `text()` | method | The rest of the body as a String. ValueError when it is not valid UTF-8, and `text() response body was already read` after the end |
| `json()` | method | The rest of the body decoded as JSON, the same as `json.decode(response)`; ValueError after the end |
| `read(size=nil)` | method | Bytes: up to `size` bytes, or everything left. Empty Bytes at the end, and on every call after it |
| `for line in response` | iteration | The rest of the body one line at a time, as it arrives: each a String without its `\n` or `\r\n`; a last line without a terminator counts |
| `close()` | method | Abandons the rest of the body and releases the connection; `nil`. Fine to call twice or after the end |
| `closed` | property | Bool: closed, or read to the end |

Reading from the network can raise NetworkError and TimeoutError; `read`,
`text`, `json` and iteration raise ValueError after `close()`.

Being readable, a Response can be handed to anything that takes `data`:
`fs.write(path, response)`, `json.decode(response)`, `csv.decode(response)`,
`hash.sha256(response)`.

Line iteration works as on a [File](./module-fs.md#file): the loop reads from
the Response and does not own it. A loop that runs to the end has read the
body to its end, which releases the connection as any complete read does. A
loop left early (`break`, `return`, an error) leaves the rest unread and the
connection held, so a stream you may leave early gets
`defer response.close()`. `read` after the loop continues where it stopped.

## Client

For several requests that share headers, a timeout, a **session** (cookies), a
redirect setting or a TLS setting. It reuses connections, as the module
functions do among themselves. It does not hold a base URL: every request
names its full URL. It shows as `<Client>`, has no properties and nothing to
close, and several goblins may use one Client.

| Member | Behaviour |
| --- | --- |
| `Client(headers=nil, timeout=30, cookies=nil, insecure=false, redirects=true)` | `headers`: sent with every request. `timeout`: the default for every request. `cookies`, `insecure`, `redirects`: below |
| `get`, `delete`, `post`, `put`, `patch`, `request` | The module functions' signatures and behaviour (`check=true` included), with the client's defaults: per-call `headers` replace the client's header of the same name (case ignored) and add the rest; a per-call `timeout`, `nil` included, replaces the client's |
| `cookies(url)` | A new Dict of name to value: the cookies the client would send to `url` (String or URL) now. No cookies is `{}`. ParseError for an invalid URL |

`check` is per call only; it is not a client setting.

**Cookies.** A Client keeps the cookies servers set (`Set-Cookie`, on redirects
too) and sends each back to the site that set it, by the browser rules for
domain, path, `Secure` and expiry. Nothing has to be asked for: log in with one
call and the next call is logged in.

- Reading is `client.cookies(url)`: for a token the site wants echoed in a
  header, and for saving a session.
- `Client(cookies={url: {name: value}})` starts with cookies: a Dict whose keys
  are URLs (String or URL) and whose values are what `cookies(url)` returns.
  Each cookie is scoped to the host of its URL, every path, only over TLS when
  the URL is `https`, and kept until the Client is dropped. A saved session is
  restored in one call; the original expiry dates are not kept. A bare
  `{name: value}` would have no site to belong to and is a TypeError
  (`Client() argument 'cookies' values must be Dict, got String`).
- As in browsers, `localhost` and loopback addresses count as secure, so a
  `Secure` cookie for them is also sent over plain `http`.
- There is no way to change or clear cookies afterwards: a session that should
  forget is a new Client. A cookie for one request only is
  `headers={"Cookie": "name=value"}`; it is sent beside the kept ones.

**`redirects=false`** returns a 3xx response as it is instead of following
it, so a script can see where a link points with
`response.headers.get("location")`. Such a response is below 400, so `check`
does not raise for it. Module functions always follow redirects.

**`insecure=true`** turns off TLS certificate verification for every
connection this Client makes, redirects included: any certificate is
accepted, for any host name, expired or not.

- What it is for: an internal or development service with a self-signed
  certificate that cannot be added to the system's store.
- What it removes: the proof that the server is the one named in the URL. The
  connection is still encrypted, but to whoever answered, so everything sent
  (tokens, cookies, bodies) can be read and changed by anyone on the path.
  Never use it for a public host.
- It is a Client setting only, so a script's trusted and untrusted traffic are
  different values. One request is `http.Client(insecure=true).get(url)`.
- The safer fix where it is possible: name the certificate authority to the
  system (`SSL_CERT_FILE` on Linux, the keychain or certificate store
  elsewhere).

## Multipart bodies

`http.multipart(fields=nil, files=nil)` encodes a form with files as
`multipart/form-data` and returns two things to pass on visibly; it sends
nothing and reads nothing.

~~~goblin
var body, content_type = http.multipart(fields={"title": "Me"}, files={"avatar": photo})
http.post(target, body=body, headers={"Content-Type": content_type})
~~~

| Returned | Value |
| --- | --- |
| `body` | A read-only [File](./module-fs.md#file) (`path` is `nil`, shows as `<File multipart>`): the encoded form, produced as it is read. Parts are read when the body is, so nothing is held in memory and every stream given as a part must still be open when the request is sent |
| `content_type` | String `multipart/form-data; boundary=…`. The boundary is random and belongs to this body; the header must be this value |

| Argument | Accepts | Becomes |
| --- | --- | --- |
| `fields` | Dict of name to value, the values `form=` accepts | One text part per field, in the Dict's order, before the files. A List value repeats the field |
| `files` | Dict of field name to a part | One file part per entry, in the Dict's order. A List of parts repeats the field |

A part in `files` is data, never a path:

| Part | File name | Content type |
| --- | --- | --- |
| An `fs.File` that has a `path` (from `fs.open`) | `file.path.name` | `application/octet-stream` |
| Dict `{"data": data, "name": String, "content_type": String}` | `name`. Required, except when `data` is an `fs.File` with a path, whose name is then the default | `content_type`; `application/octet-stream` when absent |

- `data` is a String, Bytes or readable. Only `data` is required; a key other
  than these three is a ValueError.
- A bare String, Bytes or other readable is not a part:
  `multipart() file "avatar" needs a name: pass an open fs.File, or {"name": …, "data": …}`.
  This is also what catches `files={"avatar": "photo.png"}`: a String here
  would be content, not a path.
- Nothing is guessed: the content type is not derived from the file name, and
  what is sent is what `read` gives (a File from `gzip.open` sends its
  decompressed content under the `.gz` name; give it a `name`).
- The library closes nothing. Whoever opened a file closes it, with `defer`;
  `body.close()` only stops the body and leaves the parts alone.
- The body is good for one request. Sending it again is a ValueError; to send
  again, call `multipart` again, with the files opened again.
- Empty `fields` and `files` give a valid empty form.

## How a body is sent

| `body=` | Length | Can be sent a second time (a 307 or 308 redirect) |
| --- | --- | --- |
| String, Bytes; `json=`, `form=` | `Content-Length` | Yes |
| An `fs.File` open on a regular file | `Content-Length`: from its current position to its end, measured when the request starts | No |
| The body from `http.multipart` | `Content-Length` when every part is a String, Bytes or such a File; otherwise chunked | No |
| Any other readable (a pipe, `os.stdin`, a Response, a type with `read(size)`) | Unknown: `Transfer-Encoding: chunked` | No |

- A readable is consumed by the request, whether it succeeds or fails. A second
  request needs a new readable: reopen the file.
- A length is declared only where it is known without reading, because servers
  that take uploads (object stores above all) commonly refuse a chunked
  request. A File that changes size while it is sent fails the request with a
  NetworkError.
- A 307 or 308 answer to a body that cannot be sent again is not followed: the
  Response is that answer (`status` 307, `headers["location"]`), and it does
  not raise.
- `timeout` covers the upload.

## Retrying

The library never repeats a request. A retry is a loop in the program, because
every part of it is a decision about the request: which failures pass, how long
to wait, whether a `POST` may be sent twice. The loop in the common tasks uses:

| Need | Where |
| --- | --- |
| No answer arrived | `catch e: NetworkError`, `catch e: TimeoutError` (or `catch e: IOError` for both) |
| The server refused | `catch e: http.StatusError`, then `e.status`; or `check=false` and `response.status` |
| The server's own advice | `e.response.headers.get("retry-after")`: a String, seconds (`"120"`) or an HTTP date; `nil` when absent |
| The pause | `time.sleep(seconds)` |
| The body again | String, Bytes, `json=` and `form=` values are reusable as they are. A readable or a multipart body is not: build it inside the loop |

## Left out

| Left out | What to do instead |
| --- | --- |
| A server | Not designed yet. The names `Request` and `send` are kept free for it |
| A `Request` type | A Dict expanded with `**` is the request as a value; a `Client` holds what requests share |
| A base URL, relative request URLs | An interpolation: `"{api}/users/{id}"`. `url.parse(base).join(ref)` when a link found in a page is meant |
| Retrying, in any form | A loop, shown above |
| Uploading from a path in one call; guessing a part's content type | `fs.open`, `defer`, `http.multipart`, `post`; `"content_type"` in the part |
| `auth=` | A header, shown above |
| A custom CA file, client certificates | Go reads `SSL_CERT_FILE`/`SSL_CERT_DIR` on Linux; otherwise add the CA to the system store |
| Certificate checks off for one request or module-wide | `http.Client(insecure=true)`; deliberately not shorter |
| Redirect limits and proxy configuration | Redirects are followed up to 10 or not at all (`redirects=false`); the proxy comes from the environment |
| Editing or clearing a Client's cookies; cookie attributes | A new Client is the empty session; `header_values("set-cookie")` shows what was sent |
| Decoding a multipart body; upload and download progress | No client task decodes one; read a Response with `read(size)` in a loop to follow a download |
| `HEAD`/`OPTIONS` shortcuts, `read_line()`, `Response.check()` | `request("HEAD", url)`; `for line in response`; the `check` keyword |
| Response charsets other than UTF-8 | `read()` gives the Bytes |

`timeout=` cannot cancel work in progress: a timed-out request is abandoned,
and a stream read with `timeout=nil` that the server stops feeding waits until
the connection drops.
