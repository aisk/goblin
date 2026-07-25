# Built-in functions

These names are available without an import.

| Function | Purpose |
| --- | --- |
| `print(values...)` | Write values to stdout, separated by spaces, ending with a newline |
| `eprint(values...)` | Same as `print`, but write to stderr |
| `range(start, end)` | Create integer values from start through end-exclusive |
| `min(values...)` / `max(values...)` | Choose the smallest or largest numeric value |
| `Int(value)` / `Float(value)` / `Str(value)` / `Bool(value)` | Convert a value |
| `Bytes(value)` / `List(iterable)` / `Dict(key=value, ...)` | Construct a collection value |
| `Chan([size])` | Create a channel; no size means unbuffered |
| `spawn(function, args...)` | Run a function concurrently |
| `Error(message)` | Create an error value |

`range` needs both `start` and `end`; it has no one-argument form. `min` and
`max` require at least one numeric argument. Constructors that use an
argument parser, including `range`, numeric conversions, and `Dict`, accept
their documented keyword names; `print`, `eprint`, and `spawn` are
positional-only.

Use `eprint` for diagnostics and warnings so they stay off the program's
normal stdout stream (the same role as Python's
`print(..., file=sys.stderr)`, Rust's `eprintln!`, or Go's
`fmt.Fprintln(os.Stderr, ...)`).

~~~goblin
var limits = [3, 8, 5]
print(min(*limits), max(*limits))
print(range(start=2, end=5))
print(Dict(host="127.0.0.1", port=8080))
eprint("ready on port", 8080)
~~~

See [Built-in types](./built-in-types.md) for value-specific methods, and
[Concurrency](./concurrency.md) for channels and spawn.
