# netip

The netip module wraps Go's immutable `net/netip` address and prefix values.
It performs parsing and address calculations without opening network sockets.

~~~goblin
import "netip"

var addr = netip.Addr("192.168.1.10")
var network = netip.Prefix("192.168.1.5/24")
print(addr.is_private)
print(network.masked())
print(network.contains(addr))
~~~

## Addr

Construct an address with `Addr(text)`. Its members are `bits`, `bytes`,
`zone`, `is4`, `is6`, `is_loopback`, `is_multicast`, `is_private`,
`is_unspecified`, `is_link_local_unicast`, and `is_link_local_multicast`.
Methods `next()`, `prev()`, and `unmap()` return new Addr values.

Addr values compare in Go's canonical address order. Moving beyond the first
or last address with `prev()` or `next()` raises ValueError.

## Prefix

Construct a prefix with `Prefix(text)`. It exposes `addr`, `bits`, and
`is_single_ip`, plus these methods:

| Method | Returns | Description |
| --- | --- | --- |
| `contains(addr)` | bool | Report whether the address lies in the prefix |
| `overlaps(prefix)` | bool | Report whether two prefixes overlap |
| `masked()` | Prefix | Return the canonical network prefix |
