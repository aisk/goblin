# UUID

The uuid module creates and validates UUID values using
`github.com/google/uuid`. UUIDs are a distinct Goblin type; converting one to
a string produces its canonical lowercase representation.

~~~goblin
import "uuid"

var id = uuid.new()
print(id)
print(uuid.validate("550e8400-e29b-41d4-a716-446655440000"))

var stable_id = uuid.new(
    version=5,
    namespace=uuid.namespace_dns,
    data="example.com",
)
~~~

## API

| Function | Description |
| --- | --- |
| `UUID(value)` | Constructs a UUID from a UUID, string, or 16-byte `Bytes` value. Raises `ParseError` when invalid. |
| `new(version=4, namespace=nil, data=nil)` | Creates a UUID of version 1, 3, 4, 5, 6, or 7. |
| `validate(value)` | Returns whether a string is a valid UUID representation. |

`new()` defaults to a random version 4 UUID. Versions 3 and 5 require both a
UUID `namespace` and `data`; `data` may be a string (encoded as UTF-8) or
`Bytes`. Those arguments are rejected for all other versions. The predefined
namespaces are `namespace_dns`, `namespace_url`, `namespace_oid`, and
`namespace_x500`.

UUID values expose these attributes:

| Attribute | Description |
| --- | --- |
| `bytes` | The UUID's 16 raw bytes. |
| `urn` | The UUID in `urn:uuid:...` form. |
| `version` | The numeric UUID version. |
| `variant` | The UUID variant name. |
| `time` | Creation time as a `Time`, for versions 1, 6, and 7. |
| `clock_sequence` | Clock sequence, for version 1. |
| `node` | Six node bytes, for versions 1 and 6. |

Accessing an attribute that is not defined for the UUID's version raises
`ValueError`. Functions accept both positional and keyword arguments;
`validate()` requires a string.
