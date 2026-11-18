# regex

The regex module searches and rewrites text by pattern. A pattern is a
String, written best as a raw string (`` `\d+` ``), and every function takes
it first. There is no compiled-pattern object: a pattern is compiled the first
time it is used and remembered (the most recent 256), so a pattern used in a
loop costs one lookup per call.

The syntax is RE2, the syntax of Go's `regexp` package: matching takes time
linear in the size of the text, and there are no backreferences or lookaround.
Flags are written in the pattern: `(?i)` ignores case, `(?m)` makes `^` and
`$` match at line boundaries, `(?s)` lets `.` match a newline; they combine
(`(?im)`) and can be put in front of a pattern held in a variable
(`"(?i)" + pattern`). Named groups are `(?P<name>…)` or `(?<name>…)`.

## Common tasks

Patterns are written as raw strings: backslashes and braces in them are
ordinary characters (`"\d{4}"` would be an escape error and an interpolation).

~~~goblin
import "fs"
import "json"
import "os"
import "regex"

# Does it look right
if !regex.find(`^[a-z0-9_]+$`, username) {
    raise ValueError("bad user name: {username}")
}
var is_date = regex.find(`\d{4}-\d{2}-\d{2}`, text, full=true) != nil

# Pull fields out
var m = regex.find(`(\w+)@(\w+)\.com`, "mail ada@example.com today")
if m {
    var user, domain = m.groups
    print(user, domain, m.start)              # ada example 5
}

# Parse a log line: name the fields in the pattern, then look them up
var access = `(?P<ip>\S+) \S+ \S+ \[(?P<when>[^\]]+)\] "(?P<verb>[A-Z]+) (?P<path>\S+)[^"]*" (?P<status>\d{3}) (?P<bytes>\d+|-)`
var hits = {}
var records = []
for line in fs.read_lines("access.log") {
    var entry = regex.find(access, line)
    if entry == nil {
        continue                                # not an access line
    }
    if entry["status"] == "404" {
        hits[entry["path"]] = hits.get(entry["path"], default=0) + 1
    }
    records.push(entry.to_dict())                   # {"ip": "203.0.113.9", "when": "19/Jul/2026:09:30:00 +0800", "verb": "GET", …}
}
fs.write("access.json", json.encode(records))

# Point at the part that is wrong
var setting = regex.find(`port=(?P<port>\S+)`, config_line)
var start, end = setting.span("port")
print(config_line)
print(" " * start + "^" * (end - start))

# Every occurrence
for hit in regex.find_all(`#(\w+)`, post) {
    print(hit[1])
}

# Rewrite
print(regex.replace(`\s+`, "a   b \t c", " "))                    # a b c
print(regex.replace(`(\d+)-(\d+)`, "10-20", "$2-$1"))              # 20-10
print(regex.replace(`\d+`, "a1b22", func(m) { return "<{m.text}>" }))   # a<1>b<22>

# Split on a pattern
print(regex.split(`,\s*`, "a, b,c"))                               # ["a", "b", "c"]

# Search for text that is not a pattern, ignoring case
var needle = "(?i)" + regex.escape("a.c")
print(regex.find(needle, "abc"))                                   # nil
print(regex.find(needle, "xA.Cx").text)                            # A.C

# A pattern from the command line: check it once, then use it on every line
var pattern = os.args[0]
try {
    regex.find(pattern, "")
} catch e: ParseError {
    eprint(e.message)
    os.exit(2)
}
for line in os.stdin {
    if regex.find(pattern, line) {
        print(line)
    }
}
~~~

`docs/examples/regex-tasks.goblin` runs these tasks; `examples/regex.goblin`
covers the rest of the module.

## Module functions

| Member | Returns | Behaviour |
| --- | --- | --- |
| `find(pattern, text, full=false)` | Match or `nil` | The leftmost match in `text`; `nil` when there is none. With `full=true` the pattern must match the whole of `text` |
| `find_all(pattern, text, count=nil)` | List of Match | Every non-overlapping match, left to right; at most `count`. No match is `[]` |
| `replace(pattern, text, replacement, count=nil)` | String | `text` with its matches replaced, at most `count` of them. `replacement` is a template String or a function |
| `split(pattern, text, count=nil)` | List of String | The pieces of `text` between matches; at most `count` splits. No match is `[text]` |
| `escape(text)` | String | `text` with every pattern metacharacter quoted, for building a pattern around data |

`count` means what it means on `String.split` and `String.replace`: the most
times the operation runs; `nil` is no limit, and a negative count is a
ValueError. All arguments are Strings; Bytes is a TypeError (decode first).

Empty matches follow Go: `find_all` keeps them, except one immediately after
the previous match (`regex.find_all("^|$", "ab")` finds two).

### Replacement

A template String refers to groups with `$1` or `${1}` by number and `$name`
or `${name}` by name; `$$` is a dollar sign. Write templates as raw strings
too, since `{` in a double-quoted string starts an interpolation. A reference
to a group the pattern does not have is a ValueError
(`replace() template refers to unknown group 'x'`) rather than silently
becoming empty text. `$1x` refers to a group *named* `1x`; write `${1}x` for
group 1 followed by an `x`. A `$` that starts no reference is kept as it is.

A function is called with each Match, in order, and returns the String to put
there: `func(m) { return m.text.upper() }`. A result that is not a String is a
TypeError, and an error the function raises ends `replace` and propagates.

## Match

The type name `Match` is exported for `type_of(m) == regex.Match`; calling it
raises TypeError. A Match is an immutable snapshot of one match, and always
truthy, so `if regex.find(...)` is the test for "does it match". It shows as
`<Match "ada@example.com">`, with the matched text.

| Member | Kind | Value |
| --- | --- | --- |
| `text` | property | String, what the whole pattern matched |
| `start`, `end` | property | Int positions in the searched text, **counted in characters** like every String position, so `searched[m.start:m.end] == m.text` |
| `groups` | property | List of the capture groups 1…n, each a String, or `nil` for a group that took no part. A new List on every read, so changing it does not change the Match; made for destructuring |
| `match[key]` | index | An Int selects a group by number (`0` is the whole match), a String by name. `nil` for a group that took no part |
| `to_dict()` | method | A new Dict of every **named** group, name to String (or `nil` for one that took no part), in the order the names appear in the pattern. `{}` when the pattern names none |
| `span(key)` | method | `[start, end]`: where that group matched, in characters, so `searched[start:end] == match[key]`. `key` as for `match[key]`. `nil` for a group that took no part |

`start` and `end` are the whole match's position, the common case; `span` is
for a group and is made for destructuring: `var start, end = m.span("port")`.

A group that took no part is `nil`, which keeps it apart from a group that
matched empty text (`""`). A key the pattern does not have is an IndexError
(`no such capture group: 3`); a key that is neither Int nor String is a
TypeError. When a pattern uses one name for several groups, `match[name]`,
`span(name)` and `to_dict()` all report the last of them that took part.

## Errors

| Failure | Raises | Data |
| --- | --- | --- |
| An invalid pattern, in any function | ParseError ``find() invalid pattern: missing closing ): `(` `` | `pattern`, the String as given |
| A template naming a missing group | ValueError `replace() template refers to unknown group 'x'` | — |
| A replacement function returning something other than a String | TypeError | — |
| A negative `count` | ValueError `split() argument 'count' must be non-negative, got -1` | — |
| A group the pattern does not have, in `match[key]` or `span(key)` | IndexError `no such capture group: 3` | — |

An invalid pattern raises the same ParseError on every call that uses it: a
failed compilation is not remembered. Go's parser reports no position, so
there is none. To check a pattern that comes from a user before using it,
call `regex.find(pattern, "")` inside `try` and `catch e: ParseError`.

## Left out

A compiled `Pattern` object (the cache removes the reason for one, and a
String is easier to pass around and store); a `flags=` keyword (flags are
written in the pattern); a Bool-returning `match` (`find` returns `nil` on a
miss and a Match is truthy); listing a pattern's group names without matching;
backreferences and lookaround (RE2 has neither).
