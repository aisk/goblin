# csv

The csv module turns CSV text into rows and back: reading a spreadsheet export,
writing a report a spreadsheet can open. A row is a List of Strings, or a Dict
from column name to field; a table is a List of rows. `decode` and `encode`
work on whole documents; `read_rows` reads a stream one row at a time.

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

# A file of any size, one row at a time: take the header, then loop over the rest
var big = fs.open("big.csv")
defer big.close()
var reader = csv.read_rows(big)
var columns = nil
for row in reader {
    columns = row
    break
}
for row in reader {
    var record = {}
    for i in range(columns.size) {
        record[columns[i]] = row[i]
    }
    print(record)
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
| `read_rows(readable, sep=",")` | `csv.Rows` | A lazy iterable over the rows `decode` would return for what `readable` (anything with a `read(size)` method, such as a File from `fs.open` or `gzip.open`) holds. Each step reads only as far as the next row, so the data never has to fit in memory. Reading starts where the readable is when the first loop begins, dropping a byte-order mark there. The Rows reads ahead in chunks and keeps what it read ahead, so a second loop over the same Rows continues with the row after the last one given (take the header with `break`, then loop over the rest); the readable's own position may be past that row, which matters only to code that reads the readable directly. After an error every later loop raises the same error. `read_rows` never closes the readable: whoever opened it does, with `defer`. It takes no path and no String or Bytes; for those use `decode` |
| `encode(rows, sep=",", columns=nil)` | String | CSV text, one `\n`-terminated line per row, quoting only where needed. A row whose only field is empty is written as `""`, because `decode` skips a blank line; an empty row `[]` is written as a blank line, so it is not read back. `rows` is any iterable of Lists or of Dicts. For Dicts a header line comes first: `columns` (a List of Strings) when given, otherwise the keys of the first Dict; a Dict without one of the columns writes an empty field there. With List rows, `columns` is written as the header line. No rows and no `columns` is `""` |

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
| Bytes that are not valid UTF-8, such as a Windows-1252 export | ParseError `decode() invalid CSV at line 2, column 4: invalid UTF-8` | `line`, `column` of the first invalid byte. `decode` and `decode_dicts` check the whole input before reading it; `read_rows` checks each row as it reads it, so the rows before the bad one have been yielded |
| `decode_dicts`: a record with a different number of fields than the header | ParseError `decode_dicts() invalid CSV at line 3, column 1: expected 4 fields, got 3` | `line`: the line the record starts on; `column`: 1 |
| `decode_dicts`: the header names a column twice | ParseError `decode_dicts() invalid CSV at line 1, column 8: duplicate column "id"` | `line`, `column` of the second occurrence |
| `sep` is not one usable character | ValueError `decode() argument 'sep' must be one character other than a quote or a line break, got ";;"` | — |
| Reading a readable `data` (or the `readable` of `read_rows`) fails | the error the readable raised, such as IOError | — |
| `read_rows`: `readable` has no `read(size)` method | TypeError `read_rows() argument 'readable' must have a read(size) method, got String` | — |
| `encode`: a row that is not a List or Dict | TypeError `encode() row 2 must be List or Dict, got Int` | — |
| `encode`: a List or Dict as a field, or an instance without `impl Show` | TypeError `encode() row 1 cannot hold a List as a field` | — |
| `encode`: a Dict key that is not a column | ValueError `encode() row 2 has unknown column "x"` | — |
| `encode`: a Dict row after List rows, with no `columns` | ValueError `encode() row 2 is a Dict but the rows before it are Lists; give the columns with columns=` | — |

The errors of `read_rows` name `read_rows()` and are otherwise those of
`decode`. Rows in `encode` messages are counted from 1. `line` counts lines in the text,
so a quoted field with line breaks in it moves it on; the names mean what they
mean for [json](./module-json.md).

## Left out

Reading rows by column name lazily (pair the header row with the rows of
`read_rows`, as above), comments, lenient quoting and trimming of spaces (clean
the text first), and encodings other than UTF-8.
