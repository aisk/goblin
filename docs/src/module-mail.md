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
Its `constructor` attribute is the same callable as `mail.Address`.

~~~goblin
import "mail"

var recipient = mail.parse_address("Goblin <goblin@example.com>")
print(recipient.name)
print(recipient.address)
print(Str(recipient)) # Goblin <goblin@example.com>
~~~

`Address(name, address)` does not send mail or validate that the destination
exists. It constructs a correctly formatted mailbox value. Use an empty name
for a bare address:

~~~goblin
var sender = mail.Address("", "sender@example.com")
var recipients = mail.parse_address_list(
    "Ada <ada@example.com>, goblin@example.com"
)
for recipient in recipients {
    print(recipient.name, recipient.address)
}
~~~

Address parsing accepts the syntax supported by Go's `net/mail` parser,
including quoted display names and encoded words. Invalid mailbox syntax raises
`ParseError`. The module intentionally provides address values only; composing,
parsing, or transmitting complete messages is outside its current API.
