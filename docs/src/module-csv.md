# csv

The `csv` module follows Go's `encoding/csv` package and exposes its whole-data
operations.

| Function | Go equivalent |
| --- | --- |
| `read_all(text, ...)` | `Reader.ReadAll` |
| `write_all(records, ...)` | `Writer.WriteAll` |

`read_all` returns a list of string lists. Keyword arguments configure the
corresponding Go Reader fields: `comma=","`, `comment=""`,
`fields_per_record=0`, `lazy_quotes=false`, and
`trim_leading_space=false`.

`write_all` accepts a list of string lists. It supports `comma=","` and
`use_crlf=false`, corresponding to Go's Writer fields. Parsing errors raise
`ParseError`.

By default `write_all` returns the CSV text. Passing `dest=` streams the output
into any writer object — an object with a `write(data)` method, such as an open
`fs` file — instead, and the function returns `unit`:

~~~goblin
import "csv"
import "fs"

var file = fs.create("scores.csv")
csv.write_all([["name", "score"], ["Ada", "10"]], dest=file)
file.close()
~~~

~~~goblin
import "csv"

var rows = csv.read_all("name,score\nAda,10\nGoblin,12\n")
print(rows[1][0]) # Ada

var output = csv.write_all([
    ["name", "score"],
    ["Ada", "10"],
    ["Goblin", "12"],
])
print(output)
~~~

The delimiter arguments must be a single valid character. With
`fields_per_record=0`, the reader infers the field count from the first record;
a negative value allows records of varying lengths. An empty `comment` disables
comments. CSV values are always strings: numeric conversion, header handling,
and mapping rows into dictionaries remain explicit application work.
