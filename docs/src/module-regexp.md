# regexp

The `regexp` module follows Go's RE2-based `regexp` package. Matching is linear
in the size of the input, and the syntax excludes backreferences, lookaround,
and other backtracking-only features.

~~~goblin
import "regexp"

var assignment = regexp.compile("(?P<key>[a-z]+)=(\\d+)")
print(assignment.match_string("count=12"))
print(assignment.find_string("count=12", submatch=true))
~~~

Only `Str` patterns, input, and replacements are accepted. Compile errors raise
`ParseError`.

## Module API

| Function | Go equivalent |
| --- | --- |
| `compile(pattern)` | `regexp.Compile` |
| `match_string(pattern, text)` | `regexp.MatchString` |
| `quote_meta(text)` | `regexp.QuoteMeta` |

`compile` returns an immutable `Regexp`. Converting it to a string returns the
original expression, matching Go's `Regexp.String`.

## Regexp

| Method | Go equivalent |
| --- | --- |
| `match_string(text)` | `Regexp.MatchString` |
| `find_string(text, all=false, submatch=false, index=false, n=-1)` | The `Find(All)?String(Submatch)?(Index)?` family |
| `replace_all_string(text, replacement)` | `Regexp.ReplaceAllString` |
| `split(text, n=-1)` | `Regexp.Split` |
| `subexp_names()` | `Regexp.SubexpNames` |

Goblin's keyword arguments combine Go's sixteen string-oriented `Find` methods
without changing their behavior. Set `all=true` for a list of matches,
`submatch=true` to include capture groups, and `index=true` for UTF-8 byte
offsets. The `n` argument is accepted with `all=true`: a negative value returns
all matches, zero returns none, and a positive value limits the result.

Without `all`, a plain search returns a string; an absent match is the empty
string, just as with Go's `FindString`. Index and submatch searches return
`nil` when there is no match. Unmatched subexpressions use empty strings or
`-1, -1` index pairs according to the selected representation.

`split` preserves Go's `n` rule: `n > 0` returns at most `n` substrings, `n == 0`
returns an empty list, and `n < 0` returns every substring. Replacement
templates use Go syntax such as `$1`, `${1}`, `$name`, and `${name}`.
