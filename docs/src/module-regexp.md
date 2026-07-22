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

There are no module-level matching shortcuts. Compile once and use the
resulting object, especially in loops or concurrent work.

## Pattern

| Method | Result |
| --- | --- |
| `test(text, full=false)` | Reports whether a match exists. With `full=true`, the entire text must match. |
| `find(text, full=false)` | Returns the first `Match`, or `nil`. With `full=true`, the entire text must match. |
| `find_all(text, limit=-1)` | Returns non-overlapping `Match` values. |
| `replace(text, replacement, limit=-1)` | Replaces matches using a template and returns a new string. |
| `split(text, limit=-1)` | Splits around matches and returns strings. |

`find` means leftmost substring search; the explicit `full` option avoids the
ambiguous meanings commonly attached to a method named `match`. `test` shares
exactly the same matching semantics as `find` but returns only a boolean.

All `limit` arguments have one rule: `-1` processes every match, `0` processes
none, and a positive value processes at most that many matches. For `split`, a
processed match is a separator, so `limit=1` produces at most two pieces.
Values below `-1` raise `ValueError`.

Replacement templates use Go regexp expansion syntax: `$1` and `${1}` name a
numbered group, while `$name` and `${name}` name a named group. A reference to
an unknown or unmatched group expands to the empty string. The initial API does
not support callback replacements; keeping replacement deterministic and
template-based avoids introducing a second execution and error-propagation
model. Templates are not separately parsed and therefore do not raise template
errors: following Go, a malformed `$` reference is kept as literal text, while
a well-formed unknown group reference expands to an empty string.

## Match

`Match` is an immutable snapshot. It retains the source text and copied match
indices, so it remains usable independently of later Pattern operations.

| Attribute or method | Description |
| --- | --- |
| `text` | Text matched by group 0. |
| `start`, `end` | Half-open offsets of group 0, measured in UTF-8 bytes. |
| `groups` | Numbered capture groups excluding group 0. |
| `group(index_or_name=0)` | Returns one capture by non-negative number or name. |

An optional group that did not participate is represented by `nil`, preserving
the distinction from a participating group that matched an empty string. An
unknown number or name raises `IndexError`. Group 0 is available only by number.
If a pattern repeats a capture name, name lookup returns the first participating
group with that name in numeric order; it returns `nil` when groups with that
name exist but none participated. Numbered lookup remains unambiguous.

Offsets intentionally match Go's regexp indices and Goblin's UTF-8 string
storage: they are byte offsets, not character counts. Empty matches are kept
according to Go's `FindAll` rules; an empty match immediately adjacent to a
previous match is omitted. Splitting and template expansion inherit Go regexp's
empty-match behavior.

Compiled `Pattern` values contain no Goblin-side lock. Go's `regexp.Regexp` is
safe for concurrent use, and Pattern operations do not mutate it.
