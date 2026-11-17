# csv

The csv module turns CSV text into rows and back: reading a spreadsheet export,
writing a report a spreadsheet can open. A row is a List of Strings, or a Dict
from column name to field; a table is a List of rows. It works on whole
documents.

## Common tasks

~~~goblin
import "csv"
import "fs"

# Read a spreadsheet export by column name
for person in csv.decode_dicts(fs.read("people.csv")) {
    print(person["name"], Int(person["age"]) + 1)
}

# Read plain rows, tab separated
var rows = csv.decode(fs.read("data.tsv"), sep="\t")
var header = rows[0]
var body = rows[1:]

# An export whose rows are not all the same length: read it raw, pad it yourself
for row in csv.decode(fs.read("legacy.csv"))[1:] {
    var note = ""
    if row.size > 3 {
        note = row[3]
    }
    print(row[0], note)
}

# Write rows; numbers are written as print shows them, so format the ones that need it
var table = [["name", "score"], ["Ada", 97], ["Linus", format(88.5, ".2f")]]
fs.write("scores.csv", csv.encode(table))

# Write records, choosing the column order
var people = [{"name": "Ada", "age": 36}, {"name": "Linus", "age": 28}]
fs.write("people.csv", csv.encode(people, columns=["age", "name"]))

# A file that may be broken: say where
try {
    rows = csv.decode(fs.read("upload.csv"))
} catch e: ParseError {
    eprint("upload.csv, line {e.line}: {e.message}")
}
~~~

`examples/csv.goblin` runs each of these tasks.

A program writing for a spreadsheet that wants something else has the pieces:
`sep=";"`, a byte-order mark with `(0xFEFF).to_char() + text`, Windows line
ends with `.replace("\n", "\r\n")`. One more row goes on the end of a file
with `fs.write(path, csv.encode([row]), append=true)`.

## Module members

| Member | Returns | Behaviour |
| --- | --- | --- |
| `decode(data, sep=",")` | List of List of String | Every record as a row, as long as the record is. `data` is a String, Bytes, or a readable read to its end. Quoted fields, line breaks inside them and `""` escapes follow RFC 4180. A leading byte-order mark is dropped and blank lines are skipped. Empty input is `[]` |
| `decode_dicts(data, sep=",")` | List of Dict | The first record is the header; every later record becomes a Dict from column name to field, in header order. Every record must have exactly as many fields as the header. Empty input, or a header alone, is `[]` |
| `encode(rows, sep=",", columns=nil)` | String | CSV text, one `\n`-terminated line per row, quoting only where needed. `rows` is any iterable of Lists or of Dicts. For Dicts a header line comes first: `columns` (a List of Strings) when given, otherwise the keys of the first Dict; a Dict without one of the columns writes an empty field there. With List rows, `columns` is written as the header line. No rows and no `columns` is `""` |

`sep` is one character, other than a quote or a line break.

Fields are Strings when decoding; converting a column is the program's job
(`Int(row["age"])`). When encoding, a String is written as it is, `nil` as an
empty field, and any other value as `String()` shows it (`42`, `1.5`, `true`);
an instance needs `impl Show` for that. A List or a Dict cannot be a field.

`decode` checks nothing about row lengths, so an export whose rows differ can
be read and the short rows seen (`row.size`). `decode_dicts` does check:
by name, a row with a separator missing would put values under the wrong
columns without a word.

### Errors

| Failure | Raises | Data on the error |
| --- | --- | --- |
| A quote where none may be | ParseError `decode() invalid CSV at line 3, column 9: stray quote` | `line`, `column`: Ints counted from 1, where the reader stopped; `column` counts characters |
| A quoted field that never ends | ParseError `decode() invalid CSV at line 2, column 3: quoted field never ends` | `line`, `column` |
| `decode_dicts`: a record with a different number of fields than the header | ParseError `decode_dicts() invalid CSV at line 3, column 1: expected 4 fields, got 3` | `line`: the line the record starts on; `column`: 1 |
| `decode_dicts`: the header names a column twice | ParseError `decode_dicts() invalid CSV at line 1, column 8: duplicate column "id"` | `line`, `column` of the second occurrence |
| `sep` is not one usable character | ValueError `decode() argument 'sep' must be one character other than a quote or a line break, got ";;"` | — |
| Reading a readable `data` fails | the error the readable raised, such as IOError | — |
| `encode`: a row that is not a List or Dict | TypeError `encode() row 2 must be List or Dict, got Int` | — |
| `encode`: a List or Dict as a field, or an instance without `impl Show` | TypeError `encode() row 1 cannot hold a List as a field` | — |
| `encode`: a Dict key that is not a column | ValueError `encode() row 2 has unknown column "x"` | — |
| `encode`: a Dict row after List rows, with no `columns` | ValueError `encode() row 2 is a Dict but the rows before it are Lists; give the columns with columns=` | — |

Rows in `encode` messages are counted from 1. `line` counts lines in the text,
so a quoted field with line breaks in it moves it on; the names mean what they
mean for [json](./module-json.md).

## Left out

Reading a file too large for memory row by row (until it comes, a file with no
line breaks inside quoted fields can be read with `fs.read_lines(path)` and
`csv.decode(line)[0]`), comments, lenient quoting and trimming of spaces (clean
the text first), and encodings other than UTF-8.
