# json

Import json to exchange data with JSON APIs and files. marshal(value, indent=0)
produces JSON text; unmarshal(text) parses JSON into Goblin values.

~~~goblin
import "json"

var text = json.marshal({"name": "Ada", "scores": [90, 95]})
var user = json.unmarshal(text)
print(user["name"])
print(user["scores"][0])
~~~

Objects become dictionaries, arrays become lists, JSON numbers become Int or
Float, and JSON null becomes nil. Pass a positive indent to marshal for
readable output.

~~~goblin
print(json.marshal({"ok": true}, 2))
~~~

unmarshal raises ParseError for invalid JSON. Catch it when parsing external
input.

## Values and formatting

marshal accepts every standard Goblin value that has a JSON equivalent:
dictionaries, lists, strings, integers, floats, booleans, and nil. Dictionary
keys are encoded as JSON object keys. The optional indent argument controls
pretty printing; omit it for compact data sent over a network.

~~~goblin
var payload = {
    "name": "Ada",
    "tags": ["math", "logic"],
    "enabled": true,
    "note": nil
}
print(json.marshal(payload, 2))
~~~

When decoding, inspect the resulting values with ordinary list and dictionary
operations. A JSON integer becomes Int while a decimal number becomes Float.

## Handling untrusted input

JSON from a file or HTTP response is external input. Keep parsing and
validation separate: first catch malformed JSON, then verify the fields your
program requires.

~~~goblin
try {
    var config = json.unmarshal("{\"port\": 8080}")
    var port = config.get("port", default=8080)
    print(port)
} catch err {
    if err.is(ParseError) {
        print("configuration is not valid JSON")
    } else {
        raise err
    }
}
~~~
