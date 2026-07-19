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
