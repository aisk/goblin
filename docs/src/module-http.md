# http

The http module is an HTTP client built on two messages. A `Request` is what
a client sends and a `Response` is what it receives; both have `headers` (a
`Headers`) and a `body` (always a [File](./module-fs.md#file)). Four
functions send a request:

| Function | Does |
| --- | --- |
| `exchange(request)` | One request, one response. Adds nothing, follows nothing, checks nothing |
| `send(request)` | What a user agent does: default headers, `User-Agent`, gzip, cookies, redirects, then the status check, as stated steps over `exchange` |
| `request(method, url, …)` | `send` of the `Request` built from the arguments |
| `get(url, …)` | `request("GET", url, …)` |

Anything that is a policy (which base a path is relative to, when to try
again, how a value becomes a body) is written by you, with the pieces below.

## Common tasks

~~~goblin
import "base64"
import "fs"
import "hash"
import "http"
import "json"
import "time"
import "url"

# Fetch JSON. A 404 or 500 raises, so the next line can trust what it got
var repo = json.decode(http.get("https://api.github.com/repos/aisk/goblin").body)
print(repo["stargazers_count"])

# A page as text
var page = http.get("https://example.com/").text()

# Query parameters: the url module writes the query
var query = url.encode_query({"q": "goblin lang", "page": 2})
var found = json.decode(http.get("https://api.example.com/search?{query}").body)
for item in found["items"] {
    print(item["name"])
}

# Send JSON: encode it, and say what it is
var created = http.request("POST", "https://api.example.com/users",
    headers={"Content-Type": "application/json"},
    body=json.encode({"name": "Ada"}))
print(created.status, created.headers.get("location"))

# Send a form
http.request("POST", "https://example.com/contact",
    headers={"Content-Type": "application/x-www-form-urlencoded"},
    body=url.encode_query({"email": "ada@example.com", "message": "hello"}))

# Upload a file with two form fields
var photo = fs.open("photo.png")
defer photo.close()
var form, form_type = http.multipart(
    fields={"title": "Me", "album": "2026"},
    files={"avatar": photo},
)
http.request("POST", "https://example.com/profile", headers={"Content-Type": form_type}, body=form)

# Send a big file as the body, without holding it in memory
var backup = fs.open("backup.tar.gz")
defer backup.close()
http.request("PUT", "https://storage.example.com/backup.tar.gz", body=backup, timeout=nil)

# Download to a file without holding it in memory; an error page is never saved
fs.write("goblin.tar.gz", http.get(release_url).body)

# Where did a redirect end
var landed = http.get("https://example.com/latest")
print(landed.url, landed.url.path)

# Does it exist, and how big is it, without fetching it
var head = http.request("HEAD", "https://example.com/big.iso", check=false)
print(head.status, head.headers.get("content-length"))

# Fetch only when it changed (a 304 is an answer and does not raise)
var feed = http.get(feed_url, headers={"If-None-Match": etag})
if feed.status != 304 {
    fs.write("feed.xml", feed.body)
    fs.write("feed.etag", feed.headers.get("etag", default=""))
}

# Handle a status you expect
try {
    http.request("DELETE", "https://api.example.com/users/7")
} catch e: http.StatusError {
    if e.status != 404 {
        eprint(e.response.text())
        raise e
    }
}

# A status that is an answer: wait until a job is there
var job = http.get(job_url, check=false)
while job.status == 404 {
    time.sleep(2)
    job = http.get(job_url, check=false)
}
print(json.decode(job.body)["state"])

# An API you call many times: shared headers in a Client, the address in a variable
var api = "https://api.example.com/v1"
var client = http.Client(headers={"Authorization": "Bearer {token}"}, timeout=10)
var id = json.decode(client.get("{api}/me").body)["id"]
client.request("DELETE", "{api}/sessions/current")

# A helper of your own, when a script sends JSON many times
func call(method, path, value) {
    var reply = client.request(method, "{api}{path}",
        headers={"Content-Type": "application/json"}, body=json.encode(value))
    return json.decode(reply.body)
}
call("PATCH", "/users/{id}", {"active": false})

# User name and password (Basic authentication)
var credentials = base64.encode("{user}:{password}")
var inbox = http.get("https://example.com/inbox", headers={"Authorization": "Basic {credentials}"})

# Log in, then fetch pages as that user: the Client keeps the session cookie
var shop = "https://example.com"
var site = http.Client()
var form_headers = {"Content-Type": "application/x-www-form-urlencoded"}
site.request("POST", "{shop}/login", headers=form_headers,
    body=url.encode_query({"user": "ada", "password": secret}))
var orders = site.get("{shop}/account/orders").text()

# A cookie the site wants echoed in a header
var csrf = site.cookies(shop)["csrftoken"]
site.request("POST", "{shop}/account/email",
    headers={"Content-Type": "application/x-www-form-urlencoded", "X-CSRFToken": csrf},
    body=url.encode_query({"email": "ada@example.com"}))

# Keep the session for the next run
fs.write("session.json", json.encode(site.cookies(shop)))
var saved = json.decode(fs.read("session.json"))
var resumed = http.Client(cookies={shop: saved})

# An internal service with a self-signed certificate. This gives up knowing who answers
var lab = http.Client(insecure=true)
print(json.decode(lab.get("https://build.lab.internal/status").body)["state"])

# A stream of events (one JSON document per "data:" line): each line as it arrives
var events = http.get("https://api.example.com/events", timeout=nil)
defer events.body.close()
for line in events.body {
    if line.has_prefix("data: ") {
        var event = json.decode(line.trim_prefix("data: "))
        print(event["type"])
    }
}

# A request is a value: build it, change it, send it
var ping = http.Request("POST", "https://api.example.com/ping", body="hello")
ping.headers["X-Signature"] = hash.hmac(secret, "POST /ping hello").to_hex()
http.send(ping, timeout=5)

# Try again when the network or the server has a bad moment: one function for any request
func send_patiently(request, attempts=4) {
    var pause = 1
    for attempt in range(attempts - 1) {
        try {
            return http.send(request, timeout=10)
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
    return http.send(request, timeout=10)     # the last attempt raises whatever it meets
}
var prices = json.decode(send_patiently(http.Request("GET", "https://api.example.com/prices")).body)

# Every cookie a server set, as it was sent
for cookie in landed.headers.get_all("set-cookie") {
    print(cookie.split(";")[0])
}

# Where a short link points, without following it: one exchange
var hop = http.exchange(http.Request("GET", short_url))
print(hop.status, hop.headers.get("location"))

# Follow redirects yourself, printing each step
var step = http.Request("GET", short_url)
var reply = http.exchange(step)
while [301, 302, 303, 307, 308].contains(reply.status) {
    var target = step.url.join(reply.headers["location"])
    print(reply.status, target)
    step = http.Request("GET", target)
    reply = http.exchange(step)
}

# The bytes exactly as the server sent them, compressed or not
var raw = http.get(export_url, headers={"Accept-Encoding": "identity"})
fs.write("export.bin", raw.body)
~~~

`examples/httpserver/testdata/http_tasks.goblin` runs each of these tasks
against a local server that the example tests start, through the interpreter
and as a compiled program; `examples/http.goblin` covers what needs no server.

Posting JSON or a form names a `Content-Type` and calls an encoder. That is
deliberate: the body is always the bytes you give, the header says what they
are, and JSON and query strings have one home each (`json.encode`,
`url.encode_query`). A script that does it often writes the four-line helper
above.

## Module members

| Member | Returns | Behaviour |
| --- | --- | --- |
| `Headers(fields=nil)` | Headers | [Headers](#headers) |
| `Request(method, url, headers=nil, body=nil)` | Request | [Request](#request). Sends nothing |
| `Response(status, headers=nil, body=nil)` | Response | [Response](#response). Sends nothing |
| `Client(headers=nil, timeout=30, cookies=nil, insecure=false, redirects=10)` | Client | [Client](#client) |
| `exchange(request, timeout=30)` | Response | [exchange](#exchange-one-request-one-response), on a new `Client()` |
| `send(request, timeout=30, check=true)` | Response | [send](#send-a-user-agents-request), on a new `Client()` |
| `request(method, url, headers=nil, body=nil, timeout=30, check=true)` | Response | `http.send(http.Request(method, url, headers=headers, body=body), timeout=timeout, check=check)` |
| `get(url, headers=nil, timeout=30, check=true)` | Response | `http.request("GET", url, headers=headers, timeout=timeout, check=check)` |
| `multipart(fields=nil, files=nil)` | List `[body, content_type]` | [Multipart bodies](#multipart-bodies). Sends nothing |
| `StatusError` | error kind | [Errors](#errors) |

- `request` and `get` are exactly the expressions in the table: they have no
  behaviour of their own. There is no `post`, `put`, `patch` or `delete`: the
  method is a setting, so it is the first argument of `request`.
- `Headers`, `Request`, `Response` and `Client` are constructors and are what
  `type_of(x)` returns.
- Each module function runs on a new `Client()`. A module call therefore keeps
  the cookies it is given for the length of its own redirect chain (a login
  that answers "303, here is your cookie" works) and forgets them when it
  returns; two module calls never share cookies. Connections are shared by
  every call in the process.
- Error messages name the function you called: `get() …`, `request() …`,
  `send() …`, `exchange() …`, `Request() …`.

## Headers

The header fields of one message. Names are case-insensitive and are stored in
lower case. A name may have several values. A Headers is mutable, like a Dict.

`Headers(fields=nil)` takes `nil` (empty), a Dict of String to String or to a
List of String (one name per entry, in the Dict's order; a List gives the name
several values; two keys that differ only in case are one name, their values
joined in order), or a Headers (a copy). Every `headers=` parameter of the
module accepts the same three and makes its own copy, so changing the Dict or
the Headers afterwards does not change the message.

| Member | Kind | Value |
| --- | --- | --- |
| `h[name]` | index | String: the value, or all values joined with `", "`. KeyError when the name is absent |
| `h[name] = value` | index assignment | Replaces every value of `name`. `value` is a String or a List of String; an empty List removes `name`, as `remove` does, because a field without a value is not sent. A Dict given to `Headers` (or any `headers=`) treats an empty List the same way |
| `get(name, default=nil)` | method | As `h[name]`, or `default` when the name is absent |
| `get_all(name)` | method | A new List of String: each value as it was sent or added, in order. `[]` when absent. This is the reading for `set-cookie`, whose values contain commas |
| `add(name, value)` | method | Adds one more value (a String) to `name` and keeps the ones it has. Returns the Headers |
| `remove(name)` | method | Removes the name with all its values; an absent name is accepted. Returns the Headers |
| `contains(name)` | method | Bool |
| `keys()` | method | A new List of String: the names, lower case, in order |
| `items()` | method | A new List of `[name, value]`, one pair per value, so a name with two values appears twice |
| `size` | property | Int: the number of names |
| `for name in h` | iteration | The names, as `keys()` |
| `==`, `!=` | Eq | Equal when both have the same names and each name the same values in the same order. The order of different names is ignored, as HTTP gives it no meaning. A Headers never equals a Dict |
| conditions | Truth | False when empty, as a Dict |
| `print(h)` | Show | `Headers({"content-type": "text/html", "set-cookie": ["a=1", "b=2"]})`: one value shows as a String, several as a List |

- `name` is a String in any case in every member.
- Names keep the order in which they were first added. The names of a received
  message are in alphabetical order, because Go's parser keeps them in a map;
  the values of one name are in the order they were sent.
- A Headers is not hashable and has no order. There is no `values()` (it
  would be confused with `get_all(name)`), no `copy()` (the constructor
  copies) and no `pop`, `update` or `clear`.

| Failure | Raises |
| --- | --- |
| `fields` of another type; a name that is not a String; a value that is not a String (or List of String where a List is allowed) | TypeError `Headers() argument 'fields' values must be String or List of String, got Int` |
| A name that is empty or has a character outside the RFC 9110 token set (a space, a colon, a control or non-ASCII character) | ValueError `Headers() invalid header name "a b"` |
| A value with a control character other than tab, or DEL | ValueError `Headers() invalid value for header "x-id"` |
| `h[name]` on an absent name | KeyError `key not found: "x-id"`, as a Dict |

The function named is the one used (`add() invalid header name "a b"`), or the
one whose `headers=` received the value (`request() argument 'headers' values
must be …`, `request() invalid header name "a b"`). Index assignment is syntax,
so its message names no function (`invalid header name "a b"`).

## Request

`http.Request(method, url, headers=nil, body=nil)` builds a request message.
It sends nothing: something a retry function, a signer or a logger can take.

| Argument | Accepts | Meaning |
| --- | --- | --- |
| `method` | String | Any HTTP method name, sent as written. Method names are case-sensitive: `"GET"` is the method, `"get"` a different token that servers refuse |
| `url` | String or [`url.URL`](./module-url.md#url) | Absolute, scheme `http` or `https`, without a user name or password. There is no base to resolve against: a URL built from parts is an interpolation, `"{api}/users/{id}"`. When the request is sent, a byte the request line cannot hold (a space, a non-ASCII character, a `%` that starts no escape) is percent-encoded, in the path and in the query, and so is a followed redirect's `Location`; an existing `%XX` escape and the query's delimiters (`&`, `=`, `+`, …) are sent as written, so `?q=new york` arrives as `?q=new%20york` and `?tag=a+b` as `?tag=a+b`. The `url` of the Request and of the Response show the query as written: it is encoded only on the wire |
| `headers` | `nil`, Dict, Headers | [Headers](#headers) |
| `body` | `nil`, String, Bytes, readable | The content, as it is. No `Content-Type` is added. See [How a body is sent](#how-a-body-is-sent) |

| Member | Kind | Value |
| --- | --- | --- |
| `method` | property | String |
| `url` | property | `url.URL` |
| `headers` | property | Headers. The same object on every read, so `request.headers["X-Signature"] = …` changes what is sent |
| `body` | property | File, read-only. The same object on every read. Never `nil`: a message without content has an empty stream |
| `text()` | method | `body.read().decode()`: the rest of the body as a String. ValueError `text() body is not valid UTF-8` when it is not UTF-8; `""` when the body is empty or was already read |

It shows as `<Request POST https://example.com/users>`. Its properties are
read-only; to change the method, URL or body, build a new one:
`http.Request(r.method, other, headers=r.headers, body=r.body)`.

## Response

`http.Response(status, headers=nil, body=nil)` builds a response message
(`status` is an Int from 100 to 999); `exchange`, `send`, `request` and `get`
return the one they received.

| Member | Kind | Value |
| --- | --- | --- |
| `status` | property | Int |
| `headers` | property | Headers. The same object on every read |
| `body` | property | File, read-only. The same object on every read |
| `text()` | method | As on Request |
| `url` | property | `url.URL`: the URL of the request this response answers. For `exchange` it is the request's URL; for `send` it differs when a redirect was followed. `nil` on a Response that was built |
| `version` | property | String: the HTTP version of the exchange, `"1.1"`, `"2"`, `"1.0"`. `nil` on a Response that was built |

It shows as `<Response 200>`. There is no reason phrase (HTTP/2 does not send
one) and no `success`: compare `status`. Messages compare by identity and are
not hashable.

A handler is a function from a Request to a Response, and it can be tested
without a network:

~~~goblin
func orders(request) {
    if request.method != "POST" {
        return http.Response(405, headers={"Allow": "POST"})
    }
    var order = json.decode(request.body)
    return http.Response(201, headers={"Content-Type": "application/json"},
        body=json.encode({"id": 7, "item": order["item"]}))
}
print(orders(http.Request("POST", "http://test/orders", body=json.encode({"item": "tea"}))).status)
~~~

### The body of a message

`body` is always a File, the stream type of `os.stdin` and of pipes. Its
members are those of the [fs chapter](./module-fs.md#file): `read(size=nil)`,
`read_line()`, `for line in`, `close()`, `closed`; `path` is `nil`; `write`
raises the File's ValueError.

| The message | `body` is | Shows as |
| --- | --- | --- |
| Built with `body=nil` | An empty stream | `<File body>` |
| Built with a String or Bytes | A stream over a copy of those bytes. The message keeps the bytes: reading `body` does not change what a later `send` sends | `<File body>` |
| Built with a File (this includes the body from `http.multipart` and the `body` of another message) | That File itself | as that File |
| Built with another readable (a value with `read(size)`) | A File that reads from it. Closing this File does not call the readable's `close()` | `<File body>` |
| Received | The content as it arrives from the network | `<File body>` |

Being a File, a body goes to anything that takes data:
`json.decode(response.body)`, `fs.write(path, response.body)`,
`csv.decode(response.body)`, `hash.sha256(response.body)`,
`gzip.decompress(response.body)`, and `body=` of another message. A message
is not a readable itself: `fs.write(path, response)` is the TypeError that asks
for a `read(size)` method.

**A received body and its connection.**

- The connection is released when the body has been read to its end or
  `body.close()` is called. A response whose body is read needs no `defer`.
- A loop over `response.body` that may stop early gets
  `defer response.body.close()`. After `break`, `read` continues where the
  loop stopped.
- A response dropped unread releases its connection when it is collected. A
  program that makes many requests and ignores the bodies should close them.
- `closed` is `true` only after `close()` was called.

Reading a received body can fail: NetworkError
`cannot read the response body from example.com` when the connection breaks,
and TimeoutError `timed out after 30 seconds waiting for the response body
from example.com` when no data arrives for the `timeout` of the call that
returned the Response ([Timeouts](#timeouts)). A line that is not valid UTF-8
raises the File's ValueError. These messages name no function, because the
read happens inside `read`, a `for` loop or a function such as `json.decode`
that was given the body.

## exchange: one request, one response

`client.exchange(request, timeout=<the client's>)` returns a Response;
`http.exchange(request, timeout=30)` is the same on a new `Client()`.

It sends the request to the host of its URL and returns when the status and
headers of the response have arrived. The body is not read: it is
`response.body`. The `request` object is not changed.

- The request's headers are sent as they are. No `User-Agent`, no
  `Accept-Encoding`, no cookie and no default header of the Client is added.
- The response is returned whatever its status: a 3xx is returned, and so is a
  404.
- The body is the bytes the server sent. No content coding is decoded.
- Of the Client, only the connection setting (`insecure`) and the default
  `timeout` apply.

What the library still decides in an exchange is the wire format: the
[Connection](#what-the-library-does-without-being-asked) table and
[How a body is sent](#how-a-body-is-sent). A `CONNECT` request is a ValueError
`exchange() cannot send a CONNECT request`.

## send: a user agent's request

`client.send(request, timeout=<the client's>, check=true)` returns a Response;
`http.send(request, timeout=30, check=true)` is the same on a new `Client()`.
`timeout` is one limit for all the steps ([Timeouts](#timeouts)); `check=false`
returns the Response whatever its status.

`send` does the following, in this order. This is the complete list.

1. Copies the request's headers and adds: each default header of the Client
   whose name the request does not have; `User-Agent: goblin/<version>` when
   there is none; `Accept-Encoding: gzip` when there is none, the request has
   no `Range` header and the method is not `HEAD`; and the Client's cookies
   for the URL.
2. Performs one exchange.
3. Stores the cookies the response sets.
4. When the response is a 301, 302, 303, 307 or 308 with a `Location`, fewer
   than `redirects` redirects have been followed, and the next request can be
   built: releases this response and repeats from step 1 with the next request
   ([Redirects](#redirects)).
5. When step 1 asked for gzip and the response has `Content-Encoding: gzip`:
   the body is decoded as it is read, and `content-encoding` and
   `content-length` are removed from the response's headers.
6. With `check=true`, raises `http.StatusError` when the status is 400 or
   above, or when the response is a redirect that step 4 would have followed
   and did not (the limit was reached, or a 307 or 308 asked for a stream body
   to be sent again).

The Response `send` returns is the last one. A caller who needs the steps
separately calls `exchange` and writes the ones it wants, as the redirect loop
in the common tasks does.

## Client

`http.Client(headers=nil, timeout=30, cookies=nil, insecure=false, redirects=10)`

A Client holds the settings of its connections, default headers, a cookie
store and a redirect limit. It shows as `<Client>`, has no properties and
nothing to close, and several goblins may use one Client. It does not hold a
base URL: every request names its full URL.

| Member | Behaviour |
| --- | --- |
| `exchange(request, timeout=<client's>)` | [exchange](#exchange-one-request-one-response) |
| `send(request, timeout=<client's>, check=true)` | [send](#send-a-user-agents-request) |
| `request(method, url, headers=nil, body=nil, timeout=<client's>, check=true)` | `client.send(http.Request(method, url, headers=headers, body=body), timeout=timeout, check=check)` |
| `get(url, headers=nil, timeout=<client's>, check=true)` | `client.request("GET", url, headers=headers, timeout=timeout, check=check)` |
| `cookies(url)` | A new Dict of name to value: the cookies the client would send to `url` (String or URL) now. `{}` when there are none. ParseError for an invalid URL, ValueError for one that is not an absolute http or https URL (`cookies() needs an absolute URL, got "shop"`) |

`check` is per call; it is not a Client setting. A per-call `timeout`, `nil`
included, replaces the Client's.

### What the library does without being asked

The complete list. The first table applies to `exchange` and therefore to
everything; the second applies to `send`, `request` and `get` only.

**Connection (every exchange).**

| Behaviour | Detail | To change it |
| --- | --- | --- |
| Framing | `Content-Length` or chunked encoding is computed from the body. `Content-Length`, `Transfer-Encoding`, `Connection`, `Keep-Alive`, `Proxy-Connection`, `TE` and `Upgrade` in the request's headers are not sent: they describe one connection or one wire format | — |
| Host | The request names the host of its URL. A `Host` header replaces that name; the connection still goes to the host of the URL | A `Host` header |
| Protocol version | HTTP/2 for `https` URLs when the server offers it, HTTP/1.1 otherwise | — |
| Connections | Kept and reused, shared by all Clients with the same `insecure` setting | — |
| A dead reused connection | When a request fails on a reused connection before any response arrived, it is sent once more on a new connection, if nothing of it had been written, or if its method is `GET`, `HEAD`, `OPTIONS` or `TRACE` (or it has an `Idempotency-Key` header) and its body can be sent again. This is the only case in which the library repeats a request on its own | — |
| Interim responses | 1xx responses are skipped. A request with `Expect: 100-continue` waits up to one second for the 100 before sending its body | — |
| Proxy | The one named by `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` (never for `localhost` and loopback addresses) | The environment |
| TLS verification | Certificates are verified against the system's store (`SSL_CERT_FILE`/`SSL_CERT_DIR` on Linux) | `Client(insecure=true)` |

**User agent (`send`, `request`, `get`).**

| Behaviour | Detail | To turn it off or change it |
| --- | --- | --- |
| Default headers | Each header of `Client(headers=…)` whose name the request does not have is added | `headers=nil` |
| `User-Agent: goblin/<version>` | Added when neither the request nor the Client's headers name one. The version is what `goblin --version` prints | Name one |
| gzip | `Accept-Encoding: gzip` is added when the request names no `Accept-Encoding`, has no `Range` header and is not a `HEAD`. A response with `Content-Encoding: gzip` is then decoded as it is read, and `content-encoding` and `content-length` are absent from its headers | Name an `Accept-Encoding` (for example `"identity"`), on the request or in the Client's headers. The body then arrives as the server sent it |
| Cookies | The Client keeps the cookies servers set (`Set-Cookie`, on redirects too) and sends each back to the site that set it, by the browser rules for domain, path, `Secure` and expiry. A `Cookie` header on the request is sent beside them | A new Client has none. `exchange` neither stores nor sends any |
| Redirects | Below | `Client(redirects=0)` |
| Status check | Step 6 of `send` | `check=false` |

Nothing else is added, repeated or decided. In particular the library adds no
`Referer`, no `Accept`, no `Content-Type` and no `Authorization`, and never
retries on a status.

### Redirects

A 301, 302, 303, 307 or 308 with a `Location` is followed, up to `redirects`
times.

- The next URL is the `Location` resolved against the URL of the request that
  was answered.
- 301, 302, 303: the next request is a `GET` without a body (a `HEAD` stays a
  `HEAD`), and the original request's `Content-Type`, `Content-Encoding`,
  `Content-Language` and `Content-Location` headers are not sent with it.
- 307, 308: the same method and body, when the body can be sent again. When it
  cannot (a stream), the redirect is not followed.
- The other headers of the original request are sent again, except that
  `Authorization`, `Cookie`, `Proxy-Authorization`, `WWW-Authenticate` and
  `Proxy-Authenticate` are not sent once a redirect has led to a host that is
  neither the original host nor a subdomain of it.
- `Response.url` says where the chain ended.
- `redirects` is an Int, 0 or more. With `redirects=0` every 3xx is returned
  as it is, and `check` does not raise for it: you asked to see it.
- When a redirect is not followed because `redirects` were already followed,
  or because a 307 or 308 needs a stream body again, `check=true` raises
  `http.StatusError` and `check=false` returns that 3xx response. An upload
  answered by a 307 has uploaded nothing, so it is not allowed to pass as a
  success.

### Cookies

`Client(cookies=nil)` starts with no cookies. `cookies={url: {name: value}}`
starts with cookies: the keys are URLs (String or URL) and the values are what
`cookies(url)` returns, so a saved session is restored in one call.

- Each given cookie belongs to the host of its URL, every path, only over TLS
  when the URL is `https`, and is kept until the Client is dropped. Original
  expiry dates are not restored.
- A bare `{name: value}` has no site to belong to and is a TypeError
  (`Client() argument 'cookies' values must be Dict, got String`).
- A key is checked as `Request()` checks its URL: `"example.com"`, without a
  scheme, is a ValueError (`Client() needs an absolute URL, got "example.com"`)
  rather than a site no request would match.
- A name must be a token and a value made of cookie characters (printable
  ASCII other than space, `"`, `,`, `;` and `\`), or the cookie could not be
  sent as given: ValueError `Client() invalid cookie name "a b"`,
  `Client() invalid value for cookie "sid"`.
- As in browsers, `localhost` and loopback addresses count as secure, so a
  `Secure` cookie for them is also sent over plain `http`.
- Cookies cannot be edited or cleared: a session that should forget is a new
  Client. A cookie for one request only is `headers={"Cookie": "name=value"}`.

### insecure

`insecure=true` turns off TLS certificate verification for every connection
this Client makes, redirects included. Any certificate is accepted, for any
host name, expired or not. The connection is still encrypted, but to whoever
answered, so everything sent (tokens, cookies, bodies) can be read and changed
by anyone on the path. It is for an internal or development service with a
self-signed certificate that cannot be added to the system's store, and never
for a public host. It is a Client setting only, so a script's trusted and
untrusted traffic are different values. The safer fix where it is possible:
name the certificate authority to the system (`SSL_CERT_FILE` on Linux, the
keychain or certificate store elsewhere).

| Failure | Raises |
| --- | --- |
| `redirects` negative | ValueError `Client() argument 'redirects' must be non-negative, got -1` |
| `redirects` a Bool or another type | TypeError `Client() argument 'redirects' must be Int, got Bool` |
| `cookies` keys or values of the wrong type | TypeError |
| `cookies` key not an absolute http or https URL; a cookie name or value that cannot be sent | ValueError `Client() needs an absolute URL, got "example.com"`, `Client() invalid cookie name "a b"`, `Client() invalid value for cookie "sid"` |
| `headers` | As for [Headers](#headers) |

## Timeouts

`timeout` is a number of seconds, or `nil` for no limit.

- It limits the call it is given to. For `exchange`: connecting, sending the
  request with its body, and waiting for the status and headers. For `send`,
  `request` and `get`: all of that for every exchange of the redirect chain
  together, and reading an error body before `StatusError` is raised. When the
  limit passes, the call raises TimeoutError `get() timed out after 30 seconds`.
- After the call has returned, the same number limits each wait for more of the
  response body. A body that keeps arriving is never cut off, however long the
  whole transfer takes. A body that stops arriving for longer than `timeout`
  raises TimeoutError from the read.

So a download of any size needs no `timeout=nil`. An upload that takes longer
than the limit needs a larger one or `nil`, and so does a stream of events that
can be silent for longer than the limit.

`timeout=` cannot cancel work in progress in another goblin: a timed-out
request is abandoned.

## How a body is sent

No header is derived from the body except the framing. In particular no
`Content-Type` is set.

| The request's body was built from | Length | Can be sent again (a 307 or 308, or a second `send` of the same Request) |
| --- | --- | --- |
| `nil` | None | Yes |
| String, Bytes | `Content-Length` | Yes. The Request keeps the bytes |
| A File open on a regular file | `Content-Length`: from its position to its end, measured when the request starts | No |
| The body from `http.multipart` | `Content-Length` when every part is a String, Bytes or such a File; otherwise unknown | No |
| The `body` of a received message | `Content-Length` when that message declared one and was not decoded from gzip; otherwise unknown | No |
| Any other readable (a pipe, `os.stdin`, a type with `read(size)`) | Unknown: chunked on HTTP/1.1 | No |

- A stream is read as it is sent and is never held whole. It is read to its
  end and not closed.
- A Request whose body is a stream is good for one `send` or `exchange`. A
  second one is a ValueError before anything is sent: `send() the body of
  this request is a stream that was already sent; build the request again`.
- A File that changes size while it is sent fails the request with a
  NetworkError.
- Once the call has returned or raised, the request body is not read any
  further.
- The headers of a received message can be passed to a new message unchanged,
  because the framing fields are dropped when it is sent. The `headers` of a
  received message never contain `host` (the host is in `url`) or
  `transfer-encoding`; they contain `content-length` when the server sent one.
  So a proxy is `http.exchange(http.Request(r.method, target, headers=r.headers, body=r.body))`.

## Multipart bodies

`http.multipart(fields=nil, files=nil)` encodes a form with files as
`multipart/form-data` and returns two things to pass on visibly. It sends
nothing and reads nothing.

~~~goblin
var body, content_type = http.multipart(fields={"title": "Me"}, files={"avatar": photo})
http.request("POST", target, headers={"Content-Type": content_type}, body=body)
~~~

| Returned | Value |
| --- | --- |
| `body` | A read-only File (`path` is `nil`, shows as `<File multipart>`): the encoded form, produced as it is read. Parts are read when the body is, so nothing is held in memory and every stream given as a part must still be open when the request is sent |
| `content_type` | String `multipart/form-data; boundary=…`. The boundary is random and belongs to this body; the header must be this value |

| Argument | Accepts | Becomes |
| --- | --- | --- |
| `fields` | Dict of name to value; the values `url.encode_query` accepts | One text part per field, in the Dict's order, before the files. A List value repeats the field |
| `files` | Dict of field name to a part | One file part per entry, in the Dict's order. A List of parts repeats the field |

A part in `files` is data, never a path:

| Part | File name | Content type |
| --- | --- | --- |
| A File that has a `path` (from `fs.open`) | `file.path.name` | `application/octet-stream` |
| Dict `{"data": data, "name": String, "content_type": String}` | `name`. Required, except when `data` is a File with a path, whose name is then the default | `content_type`; `application/octet-stream` when absent |

- `data` is a String, Bytes or readable. Only `data` is required; a key other
  than these three is a ValueError.
- A bare String, Bytes or other readable is not a part:
  `multipart() file "avatar" needs a name: pass an open fs.File, or {"name": …, "data": …}`.
  This is also what catches `files={"avatar": "photo.png"}`: a String here
  would be content, not a path.
- Nothing is guessed: the content type is not derived from the file name.
- In field names and file names, a carriage return, a line feed and `"` are
  written as `%0D`, `%0A` and `%22`, which is how browsers encode a form.
  Written as they are, a line break would end the part's header line and a
  quote would end the name.
- The library closes nothing. Whoever opened a file closes it, with `defer`.
- The body is good for one request. Sending it again is a ValueError
  (`send() this multipart body was already sent; build it again`); call
  `multipart` again, with the files opened again.
- Empty `fields` and `files` give a valid empty form.

## Errors

| Failure | Raises | Data |
| --- | --- | --- |
| `send`, `request`, `get` with `check=true`: the final status is 400 or above | `http.StatusError` `get() https://api.example.com/users/9 returned 404 Not Found` | `status` Int; `response` the Response |
| The same, when the redirect limit was reached | `http.StatusError` `get() https://example.com/a was redirected more than 10 times` | `status`, `response` (the last 3xx) |
| The same, when a 307 or 308 asked for a stream body again | `http.StatusError` `request() https://example.com/up answered 307 and the body is a stream that cannot be sent again` | `status`, `response` |
| No response arrived (DNS, connection, TLS) | NetworkError `get() cannot reach example.com`; the Go error is the `cause` | — |
| `timeout` passed in the call | TimeoutError `get() timed out after 30 seconds` | — |
| Reading a received body: the connection broke, or the gzip data `send` is decoding is damaged | NetworkError `cannot read the response body from example.com` | — |
| Reading a received body: no data for `timeout` | TimeoutError `timed out after 30 seconds waiting for the response body from example.com` | — |
| A readable given as `body=`, or a part of a multipart body, raises while it is sent | That error, unchanged. The request is abandoned | whatever it carries |
| A readable's length changed while it was sent | NetworkError `send() the body changed size while it was sent to example.com` | — |
| A Request with a stream body sent a second time; a multipart body that was already read from | ValueError, before anything is sent | — |
| The argument of `send` or `exchange` is not a Request | TypeError `send() argument 'request' must be Request, got Dict` | — |
| A `CONNECT` request | ValueError `send() cannot send a CONNECT request` | — |
| `url` is not a URL | ParseError `Request() invalid URL "ht!tp://x": …`, as `url.parse` | `text` |
| `url` has no scheme and host, a scheme other than `http` or `https`, or a user name or password | ValueError `Request() needs an absolute URL, got "users/7"`; `… URL scheme must be http or https, got "ftp"`; `… URL must not contain a user name or password; send an Authorization header` | — |
| `method` empty or not a token | ValueError `Request() invalid method "GET /"` | — |
| `status` outside 100–999 | ValueError `Response() argument 'status' must be between 100 and 999, got 20` | — |
| `body` of another type | TypeError `Request() argument 'body' must be String, Bytes, Nil or have a read(size) method, got Int` | — |

About `StatusError`:

- The URL in the message is the one of the request passed to the call. The
  phrase after the number is the standard one for that status; an unknown
  status has none.
- Before it is raised the rest of the body is read into memory, within the
  call's `timeout`, and the connection is released. `e.response` holds no
  resource; its `body`, `text()`, `headers`, `status` and `url` work as on any
  Response. That is everything a retry function reads: `e.status`, and
  `e.response.headers.get("retry-after")`.
- Its parent is `Error`. It is not an IOError, because the exchange worked and
  the server said no. `catch e: IOError` is "could not talk to the server"
  (NetworkError and TimeoutError are both IOErrors, and siblings: catching one
  does not catch the other). A failure of a local file is an IOError but
  neither of the two, so a retry loop that catches NetworkError and
  TimeoutError does not retry a file that cannot be read.
- `exchange` never raises it.

## Left out

| Left out | What to do instead, or where it would go |
| --- | --- |
| A server | Not designed yet. It would be a function that calls a handler with a `Request` and sends the `Response` the handler returns; both types exist |
| `post`, `put`, `patch`, `delete`, `head` | `request(method, url, …)` |
| `json=`, `form=`, `params=`, `Response.json()` | `json.encode`, `url.encode_query`, `json.decode(response.body)` |
| Content codings other than gzip | Name the `Accept-Encoding` and decode the body yourself |
| `status_text`, `success` | `response.status`; compare it |
| A base URL, relative request URLs | An interpolation, `"{api}/users/{id}"`; `url.parse(base).join(ref)` for a link found in a page |
| Retrying on a status or an error | A function over a Request, as above |
| `auth=`, a user name and password in the URL | An `Authorization` header, as above |
| Changing a message's `method`, `url` or `body` in place | Build a new one |
| Declaring the length of a stream body | A stream of unknown length is sent chunked |
| Trailers; seeing 1xx responses; an upgrade (WebSocket); `CONNECT` | Not available |
| A custom CA file, client certificates, a proxy URL, HTTP/2 without TLS, HTTP/3, pool sizes | The system store and the environment |
| Certificate checks off for one request or module-wide | `http.Client(insecure=true)`; deliberately not shorter |
| Editing or clearing a Client's cookies; cookie attributes | A new Client; `headers.get_all("set-cookie")` shows what was sent |
| Decoding a multipart body | It would come with a server |
| Header names in their original case; the order of different names on the wire | Not kept; HTTP/2 requires lower case and HTTP gives the order no meaning |
| Response charsets other than UTF-8 | `response.body.read()` gives the Bytes |
| Upload and download progress | Read `response.body` with `read(size)` in a loop; wrap a file in a type with `read(size)` |
