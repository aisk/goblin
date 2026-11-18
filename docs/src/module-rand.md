# rand

The rand module makes everyday random choices, secure random bytes and ids.
Its functions draw from one shared generator seeded differently on every run;
`rand.Rand(seed)` gives a generator of its own when the same sequence is
needed on every run, as in a test.

## Common tasks

~~~goblin
import "rand"

# Pick one, pick a few
var winner = rand.choice(entrants)
var panel = rand.sample(entrants, 3)

# Pick one where some are likelier than others
var drop = rand.choice(["common", "rare", "epic"], weights=[90, 9, 1])

# Numbers
var die = rand.int(1, 7)
var index = rand.int(items.size)
if rand.float() < 0.1 {
    print("one time in ten")
}

# Shuffle a deck
var deck = rand.shuffle(List(range(52)))

# A token that is safe to use as a secret
var token = rand.bytes(16).to_hex()

# A shorter secret that is safe in a URL or a file name (import "base64"): 24 characters
var invite = base64.encode(rand.bytes(18), url_safe=true)

# An id for a record, a file, a request
var id = rand.uuid()
fs.write("jobs/{id}.json", json.encode({"id": id, "state": "queued"}))

# Ids that sort in the order they were made
var first = rand.uuid(version=7)
var second = rand.uuid(version=7)
print(first < second)                       # true

# Is this text a UUID (import "regex")
func is_uuid(text) {
    return regex.find(`[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}`, text.lower(), full=true) != nil
}

# The same "random" data in every test run
var fixed = rand.Rand(42)
var sample_users = fixed.sample(users, 5)
print(fixed.int(100))
~~~

`docs/examples/rand-tasks.goblin` runs these tasks; `examples/rand.goblin`
covers the rest of the module.

## Module functions

| Member | Returns | Behaviour |
| --- | --- | --- |
| `int(end)` / `int(start, end)` | Int | A whole number from `start` (0 when omitted) up to but not including `end`: the numbers `range` with the same arguments would produce. `rand.int(6)` is 0–5, `rand.int(1, 7)` a die. ValueError `int() range is empty` when `end <= start` |
| `float()` | Float | From 0.0 up to but not including 1.0 |
| `choice(items, weights=nil)` | element | One element of `items` (any iterable), each equally likely. With `weights`, a List of non-negative Ints or Floats, one per item, an item is picked in proportion to its weight (`[90, 9, 1]`: nine times in ten the first) |
| `sample(items, count)` | List | `count` different elements of `items`, in random order. Different means by position, so equal values may repeat if `items` has them. `count=0` is `[]` |
| `shuffle(list)` | the same List | Puts `list` in random order in place and returns it |
| `bytes(size)` | Bytes | `size` bytes from the operating system's secure source: fit for tokens, keys and passwords. `size` is at most 1073741824 (1 GiB), because the bytes are made in memory at once |
| `uuid(version=4)` | String | A new UUID in its usual text form, 36 lower-case characters: `0190f3c2-7b1a-7d3e-9c4f-2a6b8e1d5f00`. Version 4 is random. Version 7 starts with the time in milliseconds, so ids made later sort later as plain Strings. The random bits come from the secure source |
| `Rand(seed)` | Rand | A generator of its own whose sequence is fixed by the Int `seed` |

Apart from `bytes` and `uuid`, the functions use the shared generator, which
is not suitable for secrets. Every function is safe to call from several
goblins at once.

| Failure | Raises |
| --- | --- |
| `int` with `end <= start` | ValueError `int() range is empty` |
| `choice` of an empty iterable | ValueError `choice() argument 'items' is empty` |
| `weights` of a different length, with a negative number, or all zero | ValueError |
| `sample` with a negative `count` or more than there are items | ValueError |
| `shuffle` of anything but a List | TypeError |
| A negative `bytes` size or one above 1 GiB; a `uuid` version other than 4 or 7 | ValueError (`bytes() argument 'size' must be at most 1073741824, got 2147483648`, `uuid() argument 'version' must be 4 or 7, got 1`) |
| The secure source failing | IOError |

Two version-7 ids made in the same millisecond are not ordered: the random
part follows the timestamp directly, with no counter. That is enough for
records and file names; a strict sequence is a counter, not a UUID.

## Rand

A seeded generator. Two `Rand` values with the same seed produce the same
results for the same calls, on every run and on every machine.

| Member | Behaviour |
| --- | --- |
| `int(end)` / `int(start, end)`, `float()`, `choice(items, weights=nil)`, `sample(items, count)`, `shuffle(list)` | As the module functions, drawing from this generator |

There is no `Rand.bytes` and no `Rand.uuid`: both promise unpredictability,
which a seed removes. A Rand prints as `<Rand seed=42>` and equals only
itself.

## Left out

A UUID value type and parsing UUIDs (an id is a String; normalise one from
input with `text.lower()` and check it with the pattern above); normally
distributed numbers; a random Float in a range
(`low + rand.float() * (high - low)`); several picks with repeats
(`List(range(n)).map(func(i) { return rand.choice(items) })`).
