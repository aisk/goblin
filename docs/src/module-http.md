# http

The http module makes client requests. Convenience functions return a Response
with status_code, header, body, and json() members.

~~~goblin
import "http"

var response = http.get("https://example.com")
print(response.status_code)
var text = response.body.read().decode()
response.body.close()
~~~

Use response.json() when the response body contains JSON. It consumes the body,
so choose either json() or body.read() for a response.

~~~goblin
var response = http.get("https://api.example.com/items")
var items = response.json()
~~~

post(url, content_type, body), put(), and patch() send a String, Bytes, nil,
or readable object as a request body. head(url) and delete(url) cover the
remaining common methods without a body. Use http.Client(timeout=seconds) and
its request methods when a non-default timeout is needed. Request(method, url,
body) creates a custom request for client.do().

HTTP operations can raise NetworkError; JSON response parsing can raise
ParseError. Always close a response body when it has not been fully consumed.

## Sending JSON

Use json.marshal() to build a request body and set the matching content type.
The response body is a stream, so close it after reading text or bytes.

~~~goblin
import "http"
import "json"

var payload = json.marshal({"name": "Ada"})
var response = http.post(
    "https://api.example.com/users",
    "application/json",
    payload
)
print(response.status_code)
response.body.close()
~~~

## Requests, clients, and headers

For custom methods or headers, construct Request(method, url, body=nil), then send
it through Client(timeout=seconds). Request.header supports get(), values(),
set(), add(), and del().

~~~goblin
var client = http.Client(timeout=5)
var request = http.Request("GET", "https://api.example.com/items")
request.header.set("Accept", "application/json")
var response = client.do(request)
print(response.status)
response.body.close()
~~~

The module-level functions use a finite default timeout. Treat non-success HTTP
status codes as application-level results: inspect status_code before assuming
that a response body contains the expected data.

## Reader protocol

HTTP response bodies expose `read(size=nil)`, `close()`, and the read-only
`closed` attribute. With no size, `read()` consumes all remaining bytes. With a
non-negative integer size, it returns at most that many bytes; an empty `Bytes`
value signals end of stream.

Objects supplied as request bodies use the same duck-typed protocol. They must
provide a callable `read(size)` method. Each call must return `Bytes`, `Str`, or
`nil`; an empty byte/string value or `nil` signals end of stream. A callable
`close()` method is optional and is invoked when the HTTP client closes the
request body. A request-body reader should therefore look like:

~~~goblin
type Reader(chunks) {
    func read(self, size) {
        if self.chunks.size() == 0 {
            return Bytes("")
        }
        return self.chunks.pop(0)
    }

    func close(self) {
        self.chunks.clear()
    }
}
~~~

The `size` argument is a requested upper bound. A custom reader may return a
smaller chunk, but must not require callers to omit it. `fs.File` currently has
a separate whole-file `read()` API and cannot be passed directly as an HTTP
request body.
