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
| Functions | Required parameters plus `*args` and `**kwargs`; no default parameters |
| Strings | Unicode-aware iteration and size; no `string[index]` syntax |
| Lists | Direct indexes are non-negative; some methods such as pop() accept negative indexes |
| Imports | Module scope only; local paths are relative to the importing file |
| Concurrency | Goroutines and channels; no select, cancellation, or join primitive |
| Errors | Explicit raise and try/catch; spawned-function errors are not propagated |

Goblin favors direct, explicit code over a large amount of syntax. When a
feature is absent, compose the available pieces: use a function instead of a
default parameter, a channel instead of a join operation, or an explicit loop
instead of a specialized expression form.

## Choosing a backend

Start with `goblin run` while developing. It gives direct source tracebacks and
does not need to invoke Go. Use `goblin build-exe` only once the program works;
it generates Go code and requires a working Go toolchain. A generated executable
still uses Goblin's runtime behavior, rather than turning a Goblin value into a
native Go primitive everywhere.
