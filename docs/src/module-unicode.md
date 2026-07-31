# utf8 and unicode

The utf8 module works with UTF-8 byte sequences, while unicode classifies and
maps one Unicode character at a time.

## UTF-8

| Function | Returns | Description |
| --- | --- | --- |
| `valid(data)` | bool | Check whether str or Bytes contains valid UTF-8 |
| `rune_count(data)` | int | Count decoded Unicode code points |
| `encode(codepoint)` | Bytes | Encode an integer Unicode code point |
| `decode(data)` | list | Decode the first code point as `[codepoint, byte_count]` |

`decode()` raises ValueError for empty input and ParseError when the first byte
sequence is invalid.

## Unicode characters

The predicates `is_letter`, `is_digit`, `is_number`, `is_space`, `is_upper`,
`is_lower`, and `is_control` accept a string containing exactly one Unicode
character. The mapping functions `to_upper`, `to_lower`, and `to_title` return
one mapped character.

~~~goblin
import "utf8"
import "unicode"

print(utf8.rune_count("Goblin 👺"))
print(unicode.is_letter("界"))
print(unicode.to_upper("é"))
~~~
