# Built-in functions

These names are available without an import.

| Function | Purpose |
| --- | --- |
| `print(values...)` | Write values to stdout, separated by spaces, ending with a newline |
| `eprint(values...)` | Same as `print`, but write to stderr |
| `range(end)` / `range(start, end)` | Create integer values from start (zero when omitted) through end-exclusive |
| `min(values..., key=nil)` / `max(values..., key=nil)` | Choose the smallest or largest of several values, or of one iterable's elements |
| `sum(values)` | Add up an iterable's elements with `+`, starting from `0` |
| `compare(a, b)` | Order two values as `<` does: `-1`, `0` or `1` |
| `hash(value)` | The hash a dictionary files a key under |
| `Int(value, base=10)` / `Float(value)` / `String(value)` / `Bool(value)` | Convert a value; `base` reads text in another base |
| `Bytes(value)` / `List(iterable)` / `Dict(key=value, ...)` | Construct a collection value |
| `Chan(capacity=0)` | Create a channel; capacity 0 means unbuffered |
| `Function(value)` | Validate and return a callable value unchanged |
| `spawn(function, args...)` | Run a function concurrently, returning a Goblin handle to join it |
| `wait_any(goblins, timeout=nil)` | Wait until one of several goblins finishes and return it |
| `Error(message="Error", **data)` | The base error kind; calling it creates an error of that kind, optionally carrying named data |
| `input(prompt="")` | Write a prompt to stdout and read one line from stdin, or nil at the end of input |
| `format(value, spec)` | Format a value under a format specifier, as `"{value:spec}"` does |
| `type_of(value)` | The constructor of a value's type |
| `attributes(value)` | List the names of a value's properties and methods |

`print` and `eprint` return `nil`. `range(n)` is short for `range(0, n)`;
with keywords, `start` is optional and `end` is required.

`min` and `max` order their candidates the way `<` does, so they work on
strings and on user types that implement `Ord`. Several positional arguments
are the candidates; a single one must be iterable, and its elements are
(`max([3, 8, 5])` is `8`). So pass a list itself, not `max(*values)`: expanded,
a list with one element becomes one argument that is not iterable. With no argument they raise TypeError, and with an
empty iterable ValueError. `key=` takes a function and orders the candidates
by what it returns for each, as `sort(key=)` does. On a tie `max` answers the
later candidate and `min` the earlier one, the last and first elements a
stable sort would give. `sum(values)` adds an iterable's elements with `+`
starting from `0`, so an empty one sums to `0`. `compare(a, b)` is `-1`, `0`
or `1` as `a` sorts before, with or after `b`, and raises the TypeError `<`
raises for values without an order. `hash(value)` is the hash a dictionary
files the value under, so equal values hash equal; it is stable within one
run of a program but not between runs, and a value that cannot be a key
raises TypeError (`cannot hash List`). Constructors that use an
argument parser, including `range`, numeric conversions, and `Dict`, accept
their documented keyword names; `print` and `eprint` are positional-only, and
`spawn` takes its function positionally and forwards every other argument,
keyword arguments included, to it.

`format(value, spec)` returns the String `"{value:spec}"` would give, for a
specifier built at run time (`format(name, "<{width}")`) or formatting outside
a literal (`prices.map(func(p) { return format(p, ".2f") })`). The specifier
language is described under [Formatting](./strings.md#formatting); `spec`
must be a String, an empty one gives the text `print` shows, and a malformed
one raises ValueError (`format() invalid format specifier ".2": precision
needs 'f' or 'e'`).

`input(prompt="")` writes the prompt (a String, with no newline added) to
stdout and reads one line from stdin. It returns the line without its `\n` or
`\r\n`; a last line with no terminator counts. At the end of input it
returns nil. An empty line is `""`, which is false in a condition, so test
`== nil` to detect the end. A line that is not valid UTF-8 raises ValueError.
`input()` and `os.stdin` read through one shared buffer, so they can be mixed
without losing lines.

~~~goblin
var total = 0
while true {
    var line = input()
    if line == nil { break }
    total = total + Int(line)
}
print(total)
~~~

Use `eprint` for diagnostics and warnings so they stay off the program's
normal stdout stream (the same role as Python's
`print(..., file=sys.stderr)`, Rust's `eprintln!`, or Go's
`fmt.Fprintln(os.Stderr, ...)`).

~~~goblin
var limits = [3, 8, 5]
print(min(limits), max(limits), sum(limits))
print(max(["kiwi", "fig", "banana"], key=func(s) { return s.size }))
print(range(start=2, end=5))
print(Dict(host="127.0.0.1", port=8080))
eprint("ready on port", 8080)
~~~

See [Built-in types](./built-in-types.md) for value-specific methods, and
[Concurrency](./concurrency.md) for channels and spawn.

## Constructors and type identity

`type_of(value)` returns the constructor of the value's type: the same
callable that is used to create or convert that type. A function's `name`
property says which one it is:

~~~goblin
var values = [1, "text", Bytes("raw"), [1], {"x": 1}, Chan(1)]
for value in values {
    print(type_of(value), type_of(value).name)
}

func identity(value) {
    return value
}
print(Function(identity) == identity)   # true
print(type_of(identity) == Function)    # true
print(identity.name)                    # identity
~~~

Goblin-defined instances and native standard-library values follow the same
rule: `type_of(instance)` is identical to the type callable that created them.
Every value has a type, including those that cannot be constructed: for
`nil`, a module or a Goblin handle, `type_of` returns a stand-in
function named after the type (`type_of(nil).name` is `"Nil"`) that compares
equal for values of the same type and raises TypeError when called. The type
of an error is `Error`, the base error kind: `type_of(e) == Error` holds for
every error, and calling `Error` makes a new one. It is an error rather than a
function, but it has a `name` like every other type, so `type_of(value).name`
works for any value (`type_of(e).name` is `"Error"`). Only `Error` itself has
it: on any other error `name` is free for data, as in
`NotFound("no such user", name="ann")`.

`attributes(value)` returns a new list of the names a value answers to: its
properties or fields first, then its methods, in declaration order (a
module's members are sorted by name). These are functions rather than
members, so a type is free to have a field or method called `constructor` or
`attributes`.
