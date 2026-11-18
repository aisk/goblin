# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Goblin is a toy programming language that transpiles to Go code. Source files (`.goblin`) are parsed into an AST and then converted to Go source code using the jennifer code generation library.

## Development Commands

### Building and Running
- `go build .` - Build the goblin CLI
- The CLI is cobra-based with subcommands (see `main.go`):
  - `goblin run <file.goblin>` - Interpret a source file directly via the tree-walking interpreter
  - `goblin build-exe <file.goblin>` - Transpile to Go and compile a native executable (`-o` sets output path)
  - `goblin repl` - Start an interactive REPL (history persisted to `~/.goblin_history`)
  - `goblin --version` - Print `object.Version` (`object/version.go`), which is also the http module's `User-Agent: goblin/<version>`

### Grammar Generation
- `gocc -a goblin.bnf` - Regenerate lexer, parser, token, errors, and util packages from BNF grammar

### Testing
- `go test ./...` - Run all tests
- `go test -v -run TestExamples/hello ./examples` - Run a single example test by name (`TestInterpreterExamples/hello` for the interpreter run)
- Each `examples/*.goblin` file is tested through **both backends** against the same `.stdout`/`.stderr` files: `TestExamples` builds an executable exactly as `goblin build-exe` does and runs it, `TestInterpreterExamples` interprets
- A `.stdout` file is required: if it is missing the test fails rather than generating one, so create it from a verified run when adding an example. A missing `.stderr` means "expected to be empty"
- Programs that need something only a Go test can provide live in a subdirectory of `examples/` with their own test running both backends: `closeprobe/testdata` (a test-only builtin), `exitprocess` (programs calling `os.exit`, run as real processes by `exit_process_test.go` to check stdout, stderr and the exit status on both backends) and `httpserver/testdata/http_tasks.goblin`, the http chapter's tasks against a local `httptest` server whose addresses arrive in `GOBLIN_HTTP_TEST_URL` / `GOBLIN_HTTPS_TEST_URL`

### Benchmarking
- `bench/run.sh [repeats]` - Time six compute-only benchmarks across Goblin (both backends), Go, Node, Lua and Python; prints a Markdown table
- Every implementation of a benchmark must print byte-identical output, so `bench/` also serves as a cross-backend behaviour check
- Latest measurements and the performance analysis live in `bench/RESULTS.md`

## Architecture

### Execution Flow

```
source.goblin → Lexer (lexer/) → Tokens → Parser (parser/) → AST (ast/) → semantic.CheckModule (semantic/) → { interpreter/ | transpiler/ }
```

`source.LoadFile` (`source/`) is the one entry point that reads, parses and runs `semantic.CheckModule`; both backends take an already checked `*ast.Module` and never re-check it. There are two backends over the same AST:
- **Interpreter** (`interpreter/`) — a tree-walking interpreter used by `goblin run` and the REPL (`interpreter.Run` / `interpreter.Session`). The line editor, completion and history of `goblin repl` live in `repl/`.
- **Transpiler** (`transpiler/`) — emits a Go module directory, used by `goblin build-exe` through `transpiler.BuildExecutable`. Each Goblin module becomes one Go file with an `Execute()` function (the entry module is package `main` with a `main()` calling it; imported `.goblin` modules become packages of their own). An export is a live binding in both backends: a name the module assigns anywhere (`ast.AssignedNames`) is added with `object.Module.Bind`, a getter over the interpreter's cell or the package-level Go variable, and every other export is stored in `Members` once (the interpreter's `exportModule`, the transpiler's `transpileExport`). `program.go` handles the module graph and output, a per-module `generator` (`context.go`) holds codegen state, and the remaining files split codegen by concern (`module.go`, `stmt.go`, `expr.go`, `operators.go`, `function.go`, `types.go`, `native.go`). An expression with several operands joins them through `order.go`'s `sequence`, which keeps evaluation left to right (an operand's hoisted statements run before the combined Go expression reads earlier operands); use it for any new compound form.

Runtime values use the `object.Object` interface so arithmetic and logic operations are dispatched dynamically via method calls (`.Add()`, `.Multiply()`, `.And()`, etc.). Both backends share this runtime, which is why the example tests can check them against identical expected output.

### Key Source Files (hand-written)

- `goblin.bnf` — Master grammar. All changes to syntax start here.
- `ast/` — `node.go` defines the node types, `grammar.go` the `New*`/`Append*` constructors gocc reduce actions call, and `walk.go` the shared traversal (`ast.Inspect`, `ast.WalkStatements`). Prefer these over hand-written recursive switches. Destructuring (`var a, b = v`, `a, b = v`) is one `ast.Destructure` whose `Stores` are ordinary `Declare`/`Assign`/`SetIndex`/`SetAttr` statements reading `*ast.Unpacked` values, so both backends reuse their assignment paths; `for a, b in xs` is desugared by the parser into a loop over the hidden `ast.DestructuredItem` variable whose body starts with that statement. Slicing (`x[low:high]`) is `ast.SliceExpression`, evaluated by `object.Slice` in both backends. The integer literal 9223372036854775808 is accepted only as the operand of a unary minus, which folds it into the smallest Int; `ast.CheckLiterals`, run by `source`, reports it anywhere else. `defer f(args)` (`ast.Defer`) evaluates callee and arguments at the statement and pushes the call on the `object.Defers` stack of the enclosing function or module body: the interpreter keeps it on that body's frame and runs it in `funcProto.invoke`/`runModule`/`Session.Eval`, the transpiler moves a body containing a `defer` into a closure (`wrapDeferred`) so every exit path passes through `Defers.Run`. `os.exit` is an error no catch receives (`object.NewExit`, tested with `object.IsExit`): it unwinds through the same paths, so deferred calls run, and the process ends at the top of the task (`main.go`, the generated `main`, `object.Spawn`). Library code must pass an error from Goblin code on with its chain intact.
- `transpiler/*.go` — Walks the AST and emits Go code via jennifer. Built-in functions (`print`, `eprint`, `range`, `max`, `min`, `sum`, `compare`, `hash`, `type_of`, `attributes`, `spawn`, `wait_any`) are resolved in `expr.go` through `directBuiltins`, which a test keeps in step with `extension.BuiltinsModule`. Analyses in `transpiler/{directcall,signatures,typeinfer}.go` traverse through `walkNodes` (`walk.go`, built on `ast.Inspect`), which already covers impl method bodies.
- `interpreter/*.go` — Tree-walking interpreter, REPL session, imports, and unified tracebacks. Must be kept behavior-compatible with the transpiler for the same AST. `compile.go` turns the AST into Go closures before running: locals resolve to frame slots (a block gets its own frame only when a closure inside it may capture its variables), module-level names to global cells, and positional call arguments go through per-call-site temps in the frame, so builtins must copy `CallArgs` they keep past the call.
- `semantic/semantic.go` — Semantic checks run on the module before either backend. `static.go` adds the checks that need knowledge of what a name holds: call arity against `func`/`type`/function-literal bindings and attribute names on known instances (`self` in a type's methods, a construction, a `var` initialized with one). A fact about a name only holds while the name is never assigned, so these checks are queued during the walk and run after it; their messages reuse the runtime wording of `object.BindArguments` and the instance `GetAttr`. Messages about names (`undefined identifier`, assignment to an undeclared name or a built-in) are constants in `object/names.go`, which the interpreter also raises as NameError for REPL input, since the REPL never runs the checker (`compiler.unchecked`). An example that demonstrates such a runtime error must reach it dynamically (a `*` argument, a value read back from a list).
- `object/*.go` — Runtime type system (Goblin's `Int` is the Go type `Integer`, an int64; Float is float64, Unit is nil). Each type implements the `Object` interface for arithmetic, logic, and iteration; user types dispatch through protocols (see below). Types with no operators embed `inert[...]`; unsupported operations are reported through `object.Unsupported` / `object.NewOperandError` so the wording matches across types, and argument errors name types by their `TypeName` (`String`, `Int`, …). For List, String, Dict and Bytes, `callmethod.go`'s switch is the only name-to-method mapping; `GetAttr` binds through it via each type's `methodTable`. Their members follow one specification matrix (`STDLIB_DESIGN.md` §6) pinned by `matrix_test.go`: add a new member there first. Positions resolve only through `position.go` (`ResolveIndex` for an element, `ResolveBound` for an `insert` position or a `start`, both raising IndexError out of range, and `ClampBound` for a slice end, which stops at the edge instead; all count negatives from the end), String and Bytes share their subsequence methods through `seqops.go`, and Dict keeps insertion order. A List is walked in place, by position, over at most the elements it had when the walk started, ending early once it has become shorter (`object.Iterate`); a Go loop over a list's elements that runs Goblin code for each one (a callback, a user `eq` or `show`) follows the same rule by re-reading the length at every step (`for i, n := 0, len(l.Elements); i < n && i < len(l.Elements); i++`), so a `pop` in that code never hands out a Go nil. A builtin that runs Goblin code per element of an argument walks it with `object.IterateArg` rather than `CollectArg`. Reflection (`type_of`, `attributes`) is global functions in `typeof.go`, never members. Errors (`error.go`) are kinds by identity: calling an `*Error` makes an error of that kind (`object.Call` → `(*Error).New`), keyword arguments become data read through the cause chain (`With`/`Field`), and `catch e: Kind` runs `ErrorMatches`. The global `Error` is the root kind `object.BaseError` (also what `type_of` answers for an error): `ErrorMatches` against it is true for every error except the `os.exit` request, whatever the error's chain holds, so `catch e: Error` and a bare `catch e` handle the same errors. An error no catch clause matches must leave the try exactly as if there were none: the interpreter returns the original position-tagged error, and the transpiler records which statement failed so the enclosing handler reports the same position. Equality (`equals.go`) answers `nil` against anything without calling a user `eq`, answers true for the same object without calling it, and otherwise lets the left operand's `eq` decide; the right operand's `eq` is asked only when the left has none for the pair (no `impl Eq`, or it raised TypeError), the rule `compare` follows. A NaN ranks `unordered` in `numericCompare`: `Less`/`LessEqual`/`Greater`/`GreaterEqual` answer false for it (as Go's native float comparisons in compiled code do), while `Compare`, and so `compare`, sorting, `max` and `min`, raises ValueError `cannot order NaN`. An argument that must be a function goes through `ArgParser.Func`/`NullableFunc` or `object.AsFunction`, which accept everything `Call` does (error kinds included) and report anything else as "must be callable". Inside a collection or a structural Show an error renders as `Kind("message")` and a kind (an error whose message is a capitalized name) as its name (`render.go`), the rule `PlainText` values follow. Integer arithmetic is overflow-checked through `object.Checked*` in both backends (the transpiler's unboxed fast paths included). Every Goblin function prologue calls `object.StackOK()` (compiled direct functions every `StackCheckInterval` levels via a depth parameter) so runaway recursion raises RecursionError rather than a fatal Go stack overflow; keep that in any new call path.
- `extension/builtin.go` — Built-in function implementations. Signature: `func(object.CallArgs) (object.Object, error)`. Modules are built with `object.NewModule(name, values, object.Funcs{...})`.
- `main.go` — CLI entry point (cobra commands: `run`, `build-exe`, `repl`).

### Standard Library Modules

Importable modules are implemented in Go under `extension/` and form one curated library with flat names (`os`, `json`, `http`, …). Each module is its own Go package at `extension/<module name>` exposing `Execute`, so compiled programs only link the modules they import; shared plumbing lives in `extension/internal/` (the shared value types `pathvalue`, `filestream` (File, the lazy Lines of `read_lines`, and the path-carrying errors) and `timevalue`, the JSON value mapping `jsonvalue`, the URL value and query encoding `urlvalue`, plus the `modtest` test helpers); a module never imports a sibling module's package. A new module is registered once, in `extension/stdlib/stdlib.go`: the interpreter loads executors from it, while the semantic checker and transpiler only use its names (the transpiler derives the Go import path from the name, which a stdlib test enforces, together with names being one lowercase word). API design follows `STDLIB_DESIGN.md`.

### Protocols

User types take part in operators and conversions by implementing protocols in `impl` blocks inside the type body (`impl Eq {}`, `impl Add { func add(self, other) { ... } }`). The set is closed: `Eq`, `Ord`, `Hashable`, `Show`, `Truth`, `Add`, `Sub`, `Mul`, `Div`, `Mod`, `Neg`, `Iter`, `Index`. The name after `impl` is never looked up in a scope and protocols are not values; `object.LookupProtocol` resolves it when the type is defined (semantic checker, interpreter and transpiler alike). There are no dunder methods and no user-declared protocols. All dispatch lives in `object/`: `protocols.go` defines the `Protocol` descriptors (method lists, dependencies, return types, structural implementations), and `usertype.go` holds `UserType` with its impl table indexed by protocol, the shared `CheckImpls` validation (used by both the semantic checker and runtime `Seal`), and the dispatching helpers behind `UserBase` (`userbase.go`), which both the interpreter's instances and the transpiler's generated structs embed (wired by `object.NewUser`) instead of implementing those `Object` methods themselves. Protocol methods are never instance attributes; they are reached only through their syntax, plus `compare(a, b)` and `hash(x)`. Library functions ask for ordinary methods by name instead (`read`, `write`, `close`, `to_json`). `!x` is always the negation of `ToBool`, so `Object` has no `Not` method.

### Generated Code (do not edit)

The `lexer/`, `parser/`, `token/`, `errors/`, and `util/` packages are auto-generated by `gocc goblin.bnf`. Edit the BNF grammar and regenerate instead.

### Adding New Language Features

1. Update `goblin.bnf` with new grammar rules
2. Run `gocc goblin.bnf` to regenerate parser components
3. Add AST node types and gocc constructor functions in `ast/node.go` and `ast/grammar.go` (and a case in `ast.Inspect`)
4. Implement the feature in **both** `transpiler/` and `interpreter/` — the two backends must stay behavior-compatible
5. If the feature needs a new runtime type, add it in `object/`
6. Add an example `.goblin` file under `examples/` and run tests to generate expected output
7. Update the relevant Goblin Book chapter under `docs/src/`

## Documentation (Goblin Book)

`docs/` is an mdBook (`mdbook build docs`); `docs/book/` is generated output — do not edit. Runnable programs mirrored in the book live in `docs/examples/`, verified by `docs/check-examples.sh`. Keep the book in sync when changing language behavior, built-ins, or stdlib modules.

## Key Dependencies

- `github.com/dave/jennifer/jen` — Go code generation library
- `gocc` (external CLI tool) — Parser/lexer generator from BNF grammar

## Git Commit Convention

- Using gitmoji in commit message
