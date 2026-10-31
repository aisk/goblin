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

### Grammar Generation
- `gocc -a goblin.bnf` - Regenerate lexer, parser, token, errors, and util packages from BNF grammar

### Testing
- `go test ./...` - Run all tests
- `go test -v -run TestExamples/hello ./examples` - Run a single example test by name (`TestInterpreterExamples/hello` for the interpreter run)
- Each `examples/*.goblin` file is tested through **both backends** against the same `.stdout`/`.stderr` files: `TestExamples` builds an executable exactly as `goblin build-exe` does and runs it, `TestInterpreterExamples` interprets
- A `.stdout` file is required: if it is missing the test fails rather than generating one, so create it from a verified run when adding an example. A missing `.stderr` means "expected to be empty"

### Benchmarking
- `bench/run.sh [repeats]` - Time six compute-only benchmarks across Goblin (both backends), Go, Node, Lua and Python; prints a Markdown table
- Every implementation of a benchmark must print byte-identical output, so `bench/` also serves as a cross-backend behaviour check
- Latest measurements and the performance analysis live in `bench/RESULTS.md`

## Architecture

### Execution Flow

```
source.goblin → Lexer (lexer/) → Tokens → Parser (parser/) → AST (ast/) → semantic.CheckModule (semantic/) → { interpreter/ | transpiler/ }
```

After parsing, `semantic.CheckModule` runs on the `*ast.Module` before either backend. There are two backends over the same AST:
- **Interpreter** (`interpreter/`) — a tree-walking interpreter used by `goblin run` and the REPL (`interpreter.Run` / `interpreter.Session`).
- **Transpiler** (`transpiler/`) — emits a Go module directory, used by `goblin build-exe` through `transpiler.BuildExecutable`. Each Goblin module becomes one Go file with an `Execute()` function (the entry module is package `main` with a `main()` calling it; imported `.goblin` modules become packages of their own). `program.go` handles the module graph and output, a per-module `generator` (`context.go`) holds codegen state, and the remaining files split codegen by concern (`module.go`, `stmt.go`, `expr.go`, `operators.go`, `function.go`, `types.go`, `native.go`).

Runtime values use the `object.Object` interface so arithmetic and logic operations are dispatched dynamically via method calls (`.Add()`, `.Multiply()`, `.And()`, etc.). Both backends share this runtime, which is why the example tests can check them against identical expected output.

### Key Source Files (hand-written)

- `goblin.bnf` — Master grammar. All changes to syntax start here.
- `ast/` — `node.go` defines the node types, `grammar.go` the `New*`/`Append*` constructors gocc reduce actions call, and `walk.go` the shared traversal (`ast.Inspect`, `ast.WalkStatements`). Prefer these over hand-written recursive switches.
- `transpiler/*.go` — Walks the AST and emits Go code via jennifer. Built-in functions (`print`, `eprint`, `range`, `max`, `min`) are resolved in `expr.go`. Analyses in `transpiler/{directcall,signatures,typeinfer}.go` traverse through `walkNodes` (`walk.go`, built on `ast.Inspect`), which already covers impl and trait method bodies.
- `interpreter/*.go` — Tree-walking interpreter, REPL session, imports, and unified tracebacks. Must be kept behavior-compatible with the transpiler for the same AST. `compile.go` turns the AST into Go closures before running: locals resolve to frame slots (a block gets its own frame only when a closure inside it may capture its variables), module-level names to global cells, and positional call arguments go through per-call-site temps in the frame, so builtins must copy `CallArgs` they keep past the call.
- `semantic/semantic.go` — Semantic checks run on the module before either backend.
- `object/*.go` — Runtime type system (Integer is int64, Float is float64, Unit is nil). Each type implements the `Object` interface for arithmetic, logic, and iteration; user types dispatch through traits (see below).
- `extension/builtin.go` — Built-in function implementations. Signature: `func(object.CallArgs) (object.Object, error)`.
- `main.go` — CLI entry point (cobra commands: `run`, `build-exe`, `repl`).

### Standard Library Modules

Importable modules are implemented in Go under `extension/` and come in two tiers: curated core modules with flat names (`os`, `json`, `http`, …) and direct Go wrappers registered under the `x/` prefix with Go's package hierarchy (`x/compress/gzip`, `x/crypto/sha256`, …). Each module is its own Go package at `extension/<module name>` exposing `Execute`, so compiled programs only link the modules they import; shared plumbing lives in `extension/internal/` (`digest`, `compress`, `archive`, plus the `modtest` test helpers). A new module is registered once, in `extension/stdlib/stdlib.go`: the interpreter loads executors from it, while the semantic checker and transpiler only use its names (the transpiler derives the Go import path from the name, which a stdlib test enforces). API design and tier placement follow `STDLIB_DESIGN.md`.

### Traits

User types take part in operators and conversions by implementing traits in `impl` blocks inside the type body (`impl Eq {}`, `impl Add { func add(self, other) { ... } }`); `trait Name: Deps { ... }` declares user traits, where a body-less method is required and one with a body is a default. There are no dunder methods. The built-in traits (`Eq`, `Ord`, `Hashable`, `Show`, `Truth`, `Add`, `Sub`, `Mul`, `Div`, `Mod`, `Neg`, `Iter`, `Index`) and all dispatch live in `object/`: `traits_builtin.go` defines them (method indexes, defaults, routes to `object.Object` methods for built-in values, structural implementations), `trait.go` is the trait value, and `usertype.go` holds `UserType`/`TraitImpl`, the shared `CheckImpls` validation (used by both the semantic checker and runtime `Seal`), and the trait-dispatching helpers behind `UserBase` (`userbase.go`), which both the interpreter's instances and the transpiler's generated structs embed (wired by `object.NewUser`) instead of implementing those `Object` methods themselves. Trait methods are never instance attributes; they are called as `Trait.method(receiver, ...)`. `!x` is always the negation of `ToBool`, so `Object` has no `Not` method.

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
