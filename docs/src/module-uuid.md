# UUID

The uuid module creates and parses UUIDs as defined by RFC 9562, built on Go's
standard `uuid` package. UUIDs are a distinct Goblin type; converting one to a
string produces its canonical lowercase representation.

~~~goblin
import "uuid"

var id = uuid.new()
print(id)

var ordered = uuid.new(version=7)
print(ordered.time)

var parsed = uuid.UUID("550e8400-e29b-41d4-a716-446655440000")
print(parsed.version)
~~~

## API

| Member | Description |
| --- | --- |
| `UUID(value)` | Constructs a UUID from a UUID, a string, or 16 raw `Bytes`. Raises `ParseError` when invalid. |
| `new(version=nil)` | Generates a new UUID. `version` may be `4` (random) or `7` (time-ordered). |
| `NIL` | The Nil UUID `00000000-0000-0000-0000-000000000000`. |
| `MAX` | The Max UUID `ffffffff-ffff-ffff-ffff-ffffffffffff`. |

Without a `version`, `new()` uses the algorithm Go recommends for general use,
which is currently version 4. Version 7 UUIDs embed a millisecond timestamp and
generated values sort in increasing order unless the system clock moves
backwards. Random bits always come from a cryptographically secure source.

`UUID()` accepts these string forms, with hex digits in any case:

~~~text
f81d4fae-7dec-11d0-a765-00a0c91e6bf6
{f81d4fae-7dec-11d0-a765-00a0c91e6bf6}
urn:uuid:f81d4fae-7dec-11d0-a765-00a0c91e6bf6
f81d4fae7dec11d0a76500a0c91e6bf6
~~~

There is no separate validation function. Parse the value and catch
`ParseError` instead:

~~~goblin
try {
    uuid.UUID(text)
} catch e {
    print("not a UUID")
}
~~~

## UUID values

| Property | Description |
| --- | --- |
| `bytes` | The UUID's 16 raw bytes. |
| `urn` | The UUID in `urn:uuid:...` form. |
| `version` | The version number stored in the UUID. |
| `time` | Creation time as a `Time`, for version 7 only. Raises `ValueError` otherwise. |

## Traits

UUID values implement these built-in traits:

| Trait | Behavior |
| --- | --- |
| `Eq` | Two UUIDs are equal when their 16 bytes are equal. A UUID never equals a string, even its own text form. |
| `Ord` | Ordered by big-endian byte value. Version 7 UUIDs therefore sort by creation time. Comparing with a non-UUID raises `TypeError`. |
| `Hashable` | Consistent with `Eq`, so UUIDs work as dict keys. |
| `Show` | The canonical lowercase hex-and-dash form. |

`Truth` has no UUID-specific behavior: every UUID is truthy, `NIL` included, so
compare with `uuid.NIL` explicitly to check for it. `Num`, `Iter` and `Index`
are not implemented, and using them raises `TypeError`.

Go's standard library only generates versions 4 and 7, so name-based (3, 5) and
MAC-address based (1, 6) UUIDs are not available. UUIDs of those versions can
still be parsed and report their `version`.
