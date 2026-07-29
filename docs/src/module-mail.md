# mail

The `mail` module wraps the address parsing portion of Go's `net/mail` package.
It deliberately omits message parsing: Go's message API is stream-shaped and a
larger Goblin mail-message abstraction has not been designed.

| Member | Description |
| --- | --- |
| `Address(name, address)` | Construct an address value |
| `parse_address(s)` | Parse one RFC 5322-style address |
| `parse_address_list(s)` | Parse a comma-separated address list into a `List` |

Malformed input raises `ParseError`.

An `Address` exposes read-only `name` and `address` attributes. Converting it to
text produces Go's correctly quoted and encoded mailbox representation.

~~~goblin
import "mail"

var recipient = mail.parse_address("Goblin <goblin@example.com>")
print(recipient.name)
print(recipient.address)
~~~
