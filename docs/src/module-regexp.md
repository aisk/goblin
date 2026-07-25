# regexp

The `regexp` module provides reusable regular expressions backed by Go's
RE2-based `regexp` package. Matching is linear in the size of the input. The
syntax deliberately excludes backreferences, lookaround, and other
backtracking-only features.

~~~goblin
import "regexp"

var assignment = regexp.compile("(?P<key>[a-z]+)=(\\d+)")
var match = assignment.find("count=12")
print(match.group("key"))
print(match.group(2))
~~~

Only `Str` patterns, input, and replacements are accepted. `Bytes` is not
implicitly decoded or mixed with text. Compile errors are raised as
`ParseError`, with the Go engine's diagnostic wrapped as context.

## Module API

| Function | Description |
| --- | --- |
| `compile(pattern)` | Compiles `pattern` and returns an immutable, reusable `Pattern`. |
| `escape(text)` | Quotes every metacharacter in `text`, so the result matches `text` literally. |

There are no module-level matching shortcuts. Compile once and use the
resulting object, especially in loops or concurrent work.

Use `escape` whenever part of a pattern comes from data rather than from source
code; it is the only safe way to search for text that is not itself a pattern.

~~~goblin
var needle = regexp.compile(regexp.escape("a.c"))
print(needle.match("abc"))
print(needle.match("a.c"))
~~~

## Pattern

| Attribute or method | Result |
| --- | --- |
| `pattern` | The source text this pattern was compiled from. |
| `group_names` | Capture-group names by number, excluding group 0. |
| `match(text, full=false)` | Reports whether a match exists. With `full=true`, the entire text must match. |
| `find(text, full=false)` | Returns the first `Match`, or `nil`. With `full=true`, the entire text must match. |
| `find_all(text, count=-1)` | Returns non-overlapping `Match` values. |
| `replace(text, replacement, count=-1)` | Replaces matches using a template and returns a new string. |
| `split(text, count=-1)` | Splits around matches and returns strings. |

`find` means leftmost substring search, and `match` is the same search reduced
to a boolean — the name matches `path.match`, which is also a boolean pattern
test. Requiring the whole text to match is an explicit option rather than a
separate method. The anchored engine `full=true` needs is compiled on first use,
so patterns that never ask for it pay nothing.

`group_names` lines up element for element with a `Match`'s `groups`: entry *i*
is the name of group *i + 1*, or `nil` when that group is unnamed.

`count` means exactly what it means on the built-in `str` methods, so the two
families stay interchangeable: for `split` it is the number of pieces returned,
for `replace` the number of replacements made, and for `find_all` the number of
matches returned. `0` produces nothing, and any negative value means "no limit".

~~~goblin
print(regexp.compile(",\\s*").split("a, b,c", count=2))
print("a, b,c".split(sep=", ", count=2))
~~~

Replacement templates use Go regexp expansion syntax: `$1` and `${1}` name a
numbered group, while `$name` and `${name}` name a named group, and `$$` is a
literal `$`. A reference to a group the pattern does not have raises
`ValueError` rather than silently expanding to the empty string, which would
drop text without a word. A malformed `$` that begins no reference at all is
kept as literal text, following Go.

Because `$1x` parses as a reference to a group *named* `1x`, it is rejected; use
`${1}x` to follow group 1 with literal text.

The initial API does not support callback replacements; keeping replacement
deterministic and template-based avoids introducing a second execution and
error-propagation model.

## Match

`Match` is an immutable snapshot. It retains the source text and copied match
indices, so it remains usable independently of later Pattern operations.

| Attribute or method | Description |
| --- | --- |
| `source` | The full text this match was found in. |
| `text` | Text matched by group 0. |
| `start`, `end` | Half-open offsets of group 0, measured in UTF-8 bytes. |
| `groups` | Numbered capture groups excluding group 0. |
| `named_groups` | Dict mapping each capture-group name to its text. |
| `group(key=0)` | Returns one capture by non-negative number or name. |
| `span(key=0)` | Returns the `[start, end]` offsets of one capture. |

An optional group that did not participate is represented by `nil`, preserving
the distinction from a participating group that matched an empty string. `span`
returns `nil` for the same reason. An unknown number or name raises
`IndexError`. Group 0 is available only by number. If a pattern repeats a
capture name, name lookup returns the first participating group with that name
in numeric order; it returns `nil` when groups with that name exist but none
participated. Numbered lookup remains unambiguous.

`named_groups` is a `Dict`, so its iteration order is unspecified; look names up
rather than printing it when output must be stable.

Offsets intentionally match Go's regexp indices and Goblin's UTF-8 string
storage: they are byte offsets, not character counts. Empty matches are kept
according to Go's `FindAll` rules; an empty match immediately adjacent to a
previous match is omitted. Splitting and template expansion inherit Go regexp's
empty-match behavior.

Compiled `Pattern` values contain no Goblin-side lock. Go's `regexp.Regexp` is
safe for concurrent use, and Pattern operations do not mutate it.
