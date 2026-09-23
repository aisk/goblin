# Built-in functions

These names are available without an import.

| Function | Purpose |
| --- | --- |
| `print(values...)` | Write values to stdout, separated by spaces, ending with a newline |
| `eprint(values...)` | Same as `print`, but write to stderr |
| `range(start, end)` | Create integer values from start through end-exclusive |
| `min(values...)` / `max(values...)` | Choose the smallest or largest value |
| `Int(value)` / `Float(value)` / `Str(value)` / `Bool(value)` | Convert a value |
| `Bytes(value)` / `List(iterable)` / `Dict(key=value, ...)` | Construct a collection value |
| `Chan([size])` | Create a channel; no size means unbuffered |
| `Function(value)` | Validate and return a callable value unchanged |
| `spawn(function, args...)` | Run a function concurrently, fire-and-forget |
| `Goblin(function, args...)` | Run a function concurrently, returning a joinable handle |
| `Error(message)` | Create an error value |

`print` and `eprint` return `nil`. `range` needs both `start` and `end`; it
has no one-argument form. `min` and `max` require at least one
argument and order their arguments the way `<` does, so they work on strings
and on user types that implement `Ord`. Constructors that use an
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

## Built-in traits

The traits behind operators and conversions are globals as well: `Eq`, `Ord`,
`Hashable`, `Show`, `Truth`, `Num`, `Iter` and `Index`. Types implement them in
`impl` blocks, and their methods can be called directly with the receiver
first, on any value:

~~~goblin
print(Ord.max(3, 7))            # 7
print(Eq.ne("a", "b"))          # true
print(Num.rsub(1, 10))          # 9, that is 10 - 1
~~~

See [Traits](./traits.md) for the methods of each trait.

## Constructors and type identity

Runtime values expose their constructor through `value.constructor` when the
value has a constructible type. The constructor is the same callable that is
used to create or convert that type:

~~~goblin
var values = [1, "text", Bytes("raw"), [1], {"x": 1}, Chan(1)]
for value in values {
    print(value.constructor)
}

func identity(value) {
    return value
}
print(Function(identity) == identity) # true
print(identity.constructor == Function) # true
~~~

Goblin-defined instances and native standard-library values follow the same
rule: `instance.constructor` is identical to the type callable that created
them. `nil` has no constructor because it represents the absence of a value.
