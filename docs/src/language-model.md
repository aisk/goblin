# Language model and limits

Goblin is a dynamically typed language with a tree-walking interpreter and a
Go transpilation backend. Both backends use the same parser, semantic checks,
and runtime object model. Use `goblin run` for iteration and `goblin build-exe`
when a standalone executable is needed.

The language is intentionally small. These boundaries are useful to know
before designing a larger program:

| Area | Current behavior |
| --- | --- |
| Types | Dynamic; functions and fields have no annotations |
| Static checks | Names and scopes, plus argument counts of calls to known functions and attribute names on values of a known user type; anything dynamic is checked at runtime |
| Functions | Required and defaulted parameters plus `*args` and `**kwargs` |
| Strings | Unicode-aware iteration, size, `string[index]` and `string[low:high]`, all counting characters |
| Positions | Indexes, slice bounds and method positions count from the end when negative; an out-of-range index or method position is an IndexError, while a slice takes the elements that exist between its bounds |
| Reflection | The global functions `type_of` and `attributes`; a value's members are only its fields, properties and methods |
| Protocols | A closed set of thirteen, implemented in `impl` blocks for operators and conversions; no user-declared protocols or interfaces |
| Destructuring | `var a, b = value`, `a, b = value`, and `for a, b in values` split an iterable of exactly that many elements; there are no tuples or nested patterns |
| Imports | Module scope only; local paths are relative to the importing file |
| Concurrency | `spawn` returns a joinable Goblin handle; iterable channels; `timeout=` on every blocking operation; no select or cancellation |
| Errors | Explicit raise and try/catch; a spawned function's error is re-raised by `wait()`, and reported on stderr at exit if nobody waited |
| Cleanup | `defer call()` runs the call when the enclosing function ends; there is no `finally` and no block-scoped cleanup. `os.exit()` runs the pending deferred calls of the goblin that called it; Ctrl-C ends the process without running them |

Goblin favors direct, explicit code over a large amount of syntax. When a
feature is absent, compose the available pieces: use a channel instead of a
join operation, or an explicit loop instead of a specialized expression form.

## Choosing a backend

Start with `goblin run` while developing. It gives direct source tracebacks and
does not need to invoke Go. Use `goblin build-exe` only once the program works;
it generates Go code and requires a working Go toolchain. A generated executable
still uses Goblin's runtime behavior, rather than turning a Goblin value into a
native Go primitive everywhere.
