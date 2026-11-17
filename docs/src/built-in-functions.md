# Built-in functions

These names are available without an import.

| Function | Purpose |
| --- | --- |
| `print(values...)` | Write values to stdout, separated by spaces, ending with a newline |
| `eprint(values...)` | Same as `print`, but write to stderr |
| `range(end)` / `range(start, end)` | Create integer values from start (zero when omitted) through end-exclusive |
| `min(values...)` / `max(values...)` | Choose the smallest or largest value |
| `Int(value)` / `Float(value)` / `String(value)` / `Bool(value)` | Convert a value |
| `Bytes(value)` / `List(iterable)` / `Dict(key=value, ...)` | Construct a collection value |
| `Chan(capacity=0)` | Create a channel; capacity 0 means unbuffered |
| `Function(value)` | Validate and return a callable value unchanged |
| `spawn(function, args...)` | Run a function concurrently, returning a Goblin handle to join it |
| `wait_any(goblins, timeout=nil)` | Wait until one of several goblins finishes and return it |
| `Error(message)` | Create an error value |
| `type_of(value)` | The constructor of a value's type |
| `attributes(value)` | List the names of a value's properties and methods |
| `traits(value)` | List the traits a user type's instance implements |

`print` and `eprint` return `nil`. `range(n)` is short for `range(0, n)`;
with keywords, `start` is optional and `end` is required. `min` and `max` require at least one
argument and order their arguments the way `<` does, so they work on strings
and on user types that implement `Ord`. Constructors that use an
argument parser, including `range`, numeric conversions, and `Dict`, accept
their documented keyword names; `print` and `eprint` are positional-only, and
`spawn` takes its function positionally and forwards every other argument,
keyword arguments included, to it.

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
`Hashable`, `Show`, `Truth`, `Add`, `Sub`, `Mul`, `Div`, `Mod`, `Neg`, `Iter`
and `Index`. Types implement them in
`impl` blocks, and their methods can be called directly with the receiver
first, on any value:

~~~goblin
print(Ord.max(3, 7))            # 7
print(Eq.ne("a", "b"))          # true
print(Sub.rsub(1, 10))          # 9, that is 10 - 1
~~~

See [Traits](./traits.md) for the methods of each trait.

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
`nil`, a module, a trait or a Goblin handle, `type_of` returns a stand-in
function named after the type (`type_of(nil).name` is `"Nil"`) that compares
equal for values of the same type and raises TypeError when called.

`attributes(value)` returns a new list of the names a value answers to: its
properties or fields first, then its methods, in declaration order (a
module's members are sorted by name). `traits(value)` lists the traits an
instance of a user type implements, in declaration order; any other value is
a TypeError. These are functions rather than members, so a type is free to
have a field or method called `constructor`, `attributes` or `traits`.
