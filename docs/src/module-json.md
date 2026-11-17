# json

The json module turns JSON text into Goblin values and back: loading a
configuration file, calling an API, writing a report. It has two functions,
`encode` and `decode`, and works on one whole document at a time.

## Common tasks

~~~goblin
import "fs"
import "http"
import "json"
import "os"

# Load configuration
var config = json.decode(fs.read("config.json"))
var port = config.get("port", default=8080)

# Save a report, readable by people
fs.write("report.json", json.encode(report, indent=2))

# One JSON document per line
for line in fs.read_lines("events.jsonl") {
    var event = json.decode(line)
    print(event["type"])
}

# Write one JSON document per line
var log = fs.open("events.jsonl", append=true)
defer log.close()
for event in events {
    log.write(json.encode(event) + "\n")
}

# Values JSON has no form for: choose the form where you encode
var entry = {"file": String(path), "at": started.format("%Y-%m-%d"), "size": size}
fs.write("entry.json", json.encode(entry))

# Straight from a stream
var user = json.decode(http.get("https://api.example.com/me"))

# Your own types
type Point(x, y) {
    func to_json(self) {
        return {"x": self.x, "y": self.y}
    }
}
print(json.encode([Point(1, 2)]))      # [{"x":1,"y":2}]

# Bad input: say where
try {
    config = json.decode(fs.read("config.json"))
} catch e: ParseError {
    eprint("config.json:{e.line}:{e.column}: {e.message}")
    os.exit(1)
}
~~~

`examples/json.goblin` runs each of these tasks except the request, which
`examples/httpserver/testdata/http_tasks.goblin` runs against a local server.
A Response is a readable, so `json.decode` reads it to its end; the
[http](./module-http.md) chapter's `response.json()` is the same thing as a
method.

JSON text written inside a Goblin string needs a raw string (`` `{"a": 1}` ``)
or escaped braces (`"\{\"a\": 1}"`), because `{` starts an interpolation.

## Module members

| Member | Returns | Behaviour |
| --- | --- | --- |
| `encode(value, indent=nil)` | String | The JSON text of `value`. `indent=nil` is compact (`{"a":1,"b":[1,2]}`); an Int is the number of spaces per level, with `": "` after keys. Dict entries are written in the Dict's order. In strings only `"`, `\` and the control characters below U+0020 are escaped, so `<`, `>`, `&` and every other character are written as they are |
| `decode(data)` | value | The value of one JSON document. `data` is a String, Bytes, or a readable read to its end. Objects become Dicts in document order |

## Values

| Goblin | JSON | Back |
| --- | --- | --- |
| `nil` | `null` | `nil` |
| Bool | `true` / `false` | Bool |
| Int | number | Int when the number has no fraction or exponent and fits 64 bits |
| Float | number (the shortest form that reads back the same; a whole Float keeps `.0`) | Float otherwise |
| String | string | String |
| List | array | List |
| Dict with String keys | object | Dict |
| any value with a `to_json()` method | whatever `to_json()` returns, encoded in turn | — |
| anything else (Bytes, Path, Time, an instance without `to_json()`, …) | TypeError | — |

`to_json()` is an ordinary method, one of the methods the library asks for by
name: nothing to import or declare. It takes no arguments, must return
something encodable, and must not return the value itself. Library value types
do not have it; convert them where you encode (`String(path)`,
`t.format(layout)`, `base64.encode(bytes)`), so the file says which form it
holds.

A key that appears twice in a JSON object keeps its last value, at the place
of its first occurrence.

### Errors

| Failure | Raises | Data on the error |
| --- | --- | --- |
| `decode`: text that is not one JSON document, including empty text and anything but whitespace after the document | ParseError `decode() invalid JSON at line 2, column 9: unexpected "}"` | `line`, `column`: Ints counted from 1, where the text stops being JSON. `column` counts characters, not bytes. Empty input is line 1, column 1 |
| `decode`: bytes that are not valid UTF-8 | ParseError `decode() invalid JSON at line 1, column 2: invalid UTF-8` | `line`, `column` of the first invalid byte |
| `decode`: a number too large for a Float | ParseError `decode() invalid JSON at line 1, column 2: number 1e999 is out of range` | `line`, `column` |
| `decode`: `data` has the wrong type | TypeError `decode() argument 'data' must be String, Bytes or have a read(size) method, got Int` | — |
| `decode`: reading a readable `data` fails | the error the readable raised, such as IOError | — |
| `encode`: a value with no JSON form | TypeError `encode() cannot encode Chan (give its type a to_json() method)` | — |
| `encode`: a Dict key that is not a String | TypeError `encode() Dict keys must be String, got Int` | — |
| `encode`: NaN or infinity | ValueError `encode() cannot encode +Inf (JSON has no NaN or infinity)` | — |
| `encode`: a negative `indent` | ValueError `encode() argument 'indent' must be non-negative, got -1` | — |
| `encode`: `to_json()` returns the value itself | ValueError `encode() Point.to_json() returned the value itself` | — |
| `encode`: a value that contains itself | RecursionError | — |

The message already says where; `line` and `column` are for a program that
reports the place its own way, as `config.json:2:9`.

## Left out

Sorted keys (keys keep the Dict's order; build the Dict from
`d.keys().sort()` when a diff needs it), streaming a document too large for
memory (use one document per line, as above), comments and trailing commas
(that is not JSON), and automatic encoding of a type's fields (write
`to_json()`).
