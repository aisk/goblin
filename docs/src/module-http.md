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
or readable object as a request body. Use http.Client(timeout=seconds) and its
request methods when a non-default timeout is needed. Request(method, url,
body) creates a custom request for client.do().

HTTP operations can raise NetworkError; JSON response parsing can raise
ParseError. Always close a response body when it has not been fully consumed.
