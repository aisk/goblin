# Goblin Standard Library Design Guidelines

This document defines how standard library modules under `extension/` are designed and implemented. It governs API shape and conventions; the mechanics of wiring a module up (registration, docs, tests) are covered in `CLAUDE.md`.

Existing modules are cited below as illustrations, not as canon: they predate this document and may themselves get things wrong. When an existing module conflicts with these guidelines, the guidelines win — treat the conflict as a bug to fix (or raise for review if the fix is breaking), not as precedent to imitate.

## 1. Source of truth: the Go standard library

- Goblin's stdlib is modelled on the Go standard library. When deciding what a module should contain and what its functions should be called, start from the corresponding Go package and stay conceptually close to it.
- "Near-standard" Go libraries are acceptable as a base too: packages under `golang.org/x/*`, or de-facto standards maintained by trusted stewards with a stable API (e.g. `github.com/google/uuid`). Anything else needs human review before being added as a dependency.
- Module names are flat, lowercase, and drop Go's package hierarchy: `encoding/json` → `json`, `compress/gzip` → `gzip`, `archive/tar` → `tar`, `crypto/sha256` → `sha256`. The mapping is per registered module, not per Go source file (`compression.go` implements the `gzip` and `zlib` modules).
- Function and method names are lowercase (snake_case when multi-word); type names are Capitalized (`Path`, `UUID`, `File`). Snake-casing a Go name whose words all carry meaning is fine (`csv.read_all` ← `ReadAll`, `s.to_title` ← `strings.ToTitle`); what gets dropped are the parts of a Go name that only exist to tell overload-family variants apart (`ParseInt`'s `Int`, `EncodeToString`'s `ToString` vs buffer-writing `Encode`) — once §3 collapses the family into one function, the discriminating suffix has nothing left to discriminate. The binding rule is cross-module consistency: the same concept gets the same name everywhere. For example, `hex.encode` was formerly exposed as `hex.encode_to_string`, despite naming the same concept as `base64.encode`; dropping the Go-specific `to_string` suffix fixed that inconsistency.
- Module-level constants are UPPER_CASE snake_case (`exec.INHERIT`, `gzip.BEST_SPEED`, `uuid.NAMESPACE_DNS`), visually distinct from functions and methods.
- Snake-casing applies to names that are genuinely multi-word in Goblin's vocabulary. POSIX-heritage identifiers that read as single lexemes (`getenv`, `getpid`, `getwd`, …) are kept as-is, not force-segmented. And when the Go name itself is awkward, choosing a deliberately different, better name is allowed with review — `os.tempdir`/`os.tempfile` ← `os.MkdirTemp`/`os.CreateTemp` is the precedent.

## 2. When to depart from Go: object-oriented wrapping

Go's older, imperative APIs (designed before the language had richer idioms) should be re-shaped into object-oriented APIs when the domain has a natural central type. `path.Path` is the template: the module exposes a type plus a few factory functions (`Path(...)`, `cwd()`, `home()`), and everything that operates on a value becomes a method on it.

Apply this when:

- The Go API is a bag of free functions that all take the same first argument (`filepath.Join(p, ...)`, `filepath.Base(p)`, …) — that argument wants to be a receiver.
- A well-known OO precedent exists in another mainstream language (e.g. Python's `pathlib` for `path`); borrowing its shape is encouraged as long as the underlying semantics remain Go's.

Do **not** invent object wrappers for APIs that are already value-oriented or function-shaped in a good way (`math`, `base64`, `hex`): keep those as plain module functions mirroring Go.

Module-level members should be only what cannot be a method — constructors, factories, and true module-level operations. If a function could be a method on an existing type, it must be. (Applied literally this indicts most of today's `fs` module, whose path-first free functions overlap with `path.Path`; whether `fs` migrates to Path methods or keeps a procedural convenience layer is an open question for human review, not something to resolve module-by-module in passing.)

## 3. Merging Go's overload families

Go has no default arguments or overloading, so it grows function families like `Split`/`SplitN`, `Encode`/`EncodeToString`, `NewWriter`/`NewWriterLevel`. Goblin call sites support positional and keyword arguments plus `*`/`**` unpacking, and Go-implemented stdlib functions can give any parameter a default via `ArgParser` — use that to collapse each family into **one** function.

- The common case must work with only positional required arguments: `s.split(sep)`.
- Variations become optional keyword arguments with defaults that reproduce the Go zero-config behaviour: `s.split(sep, count=-1)`, `gzip.compress(data, level=gzip.DEFAULT_COMPRESSION)`.
- Never expose `foo` and `foo_with_bar` side by side when a keyword argument can express the difference.
- Collapse configuration, not concepts: when a shared prefix/suffix names a genuinely different operation rather than a config knob, the functions stay separate. `url.query_escape` and `url.path_escape` apply different encoding rules to different URL components — folding them into `escape(s, mode=...)` would trade two self-describing names for a stringly-typed enum. The test: if the variants differ in *what* they do (not just in a parameter Go couldn't default), keep them apart.
- Keyword names are part of the API — choose them as carefully as function names, and reuse the same name for the same concept across modules (`level`, `sep`, `count`, `comma`, …).

All argument handling goes through `object.ArgParser` (`object/argparse.go`): it defines positional order, keyword precedence, defaults, and uniform `TypeError` messages. Do not hand-roll argument validation in new code — roughly half the existing modules predate `ArgParser` and validate by hand; that is legacy to migrate, not a second acceptable style. For parameter types `ArgParser` has no typed accessor for yet (Bytes, List, Dict), use `Any`/`AnyOr` plus a manual type assertion that keeps the standard message format (`funcname() argument 'x' must be ..., got ...`) — or better, add the missing accessor.

## 4. Errors

- Never return raw Go errors and never panic across the boundary. Wrap native errors with `object.WrapNativeError` / `object.WrapError`, attaching the appropriate sentinel from the hierarchy in `object/error.go` (`IOError`, `ParseError`, `PermissionError`, …).
- Add a new sentinel error kind only when callers realistically need to catch that case distinctly; otherwise use the nearest existing parent.
- Error messages follow the existing format: `funcname() <what failed>`. A bare `funcname() failed` says nothing — name the failure (`decode() invalid hex data`, `read() failed to read HTTP body`).

## 5. Values, not Go internals

- Accept and return Goblin runtime types (`object/`). Go-specific machinery — channels, `context.Context`, struct configs, interfaces-as-extension-points — must not leak into a module's API as-is.
- Ubiquitous Go interfaces like `io.Reader`/`io.Writer` are the exception: the *concept* (a stream you can read from / write to) is worth keeping, expressed through duck typing rather than a declared interface. There is no predefined `Reader` type to import or implement — the method shape *is* the contract, and stdlib functions that consume streams accept any object providing it (the mechanism exists: extension code can call user-object methods via `GetAttr`, symmetric in both backends; `http`'s body reader is the precedent). The canonical reader shape, matching what `http` produces and consumes today: `read(size)` where `size` is a non-negative int (a producer may additionally allow calling with no argument to read everything); it returns a chunk as Bytes (consumers should also tolerate str); end of stream is signalled by returning an **empty chunk or `nil` — consumers must accept both**, and producers should return an empty Bytes like `http.Body` does. No writer protocol is defined yet — there is no consumer of one in the stdlib; the writer shape gets specified (with human review) when the first consumer appears, not preemptively. New stream-shaped APIs must follow the canonical shape, must not invent a variant, and must document the expected shape in user-facing docs — a Go-side comment is not documentation (known debt: the Goblin Book `http` chapter itself does not yet document this shape). Existing non-conforming streams (e.g. `fs.File.read()`, which takes no arguments and reads the whole file) fall under the general rule above: bugs to fix, not variants to accommodate.
- APIs whose Go counterpart takes a `context.Context` are wrapped **without** any context parameter for now: pass `context.TODO()` internally and do not invent ad-hoc timeout/cancellation arguments. Mirroring a timeout Go itself exposes as plain configuration (e.g. `http.Client(timeout=...)` ← `http.Client.Timeout`) is fine — the ban is on per-call cancellation plumbing, not on native config knobs. The long-term plan is a Thread-style object wrapping a goroutine that exposes timeout and message-passing on itself, so cancellation scopes the thread rather than being threaded through every call; per-call context plumbing added today would only have to be unwound then.
- Where Go returns `[]byte` vs `string` variants, pick the one natural Goblin type and provide the other via an argument or method only if genuinely needed.

## 6. Skip and flag for review

Some Go APIs are too abstraction-heavy to wrap simply: many interacting concepts, config structs, interface-based extension points (e.g. large parts of `net/http`'s server side, `reflect`, `database/sql` drivers). For these:

- Wrap the useful, simple 80% (as `http` does for client requests) and **skip the rest** rather than shipping a distorted or half-faithful abstraction.
- Every deliberate omission or nontrivial deviation from the Go API must be called out for human review — in the PR description and, when user-visible, in the module's Goblin Book chapter — instead of being silently dropped.
- A wrong abstraction shipped is worse than a missing one: when in doubt, leave it out and note it.

## 7. Checklist for a new module

1. Identify the Go (or near-standard) package it maps to; note every intentional deviation.
2. Decide the shape: plain functions (Go-like) or central type + factories (`path.Path`-like), per §2.
3. Collapse overload families using keyword arguments with defaults, per §3.
4. Implement with `ArgParser` and the sentinel error hierarchy.
5. Register in both backends, add `examples/`, tests, and a Goblin Book chapter (see `CLAUDE.md`).
6. List skipped/deviating surface area in the PR for review, per §6.
