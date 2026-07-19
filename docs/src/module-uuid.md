# UUID

The uuid module creates and validates RFC 4122 UUID values using
`github.com/google/uuid`. UUIDs are a distinct Goblin type; converting one to
a string produces its canonical lowercase representation.

~~~goblin
import "uuid"

var id = uuid.new()
print(id)
print(uuid.validate(id))
~~~

## API

| Function | Description |
| --- | --- |
| `new()` | Creates a random version 4 UUID value. |
| `parse(value)` | Parses a UUID string and returns a UUID value. Raises `ParseError` when invalid. |
| `validate(value)` | Returns whether `value` is a valid UUID value or string. |

UUID values expose `version` and `variant` attributes. All functions are
positional-only. `parse()` requires a string; `validate()` accepts a UUID or
string.
