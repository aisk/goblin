# Goblin Standard Library Design Guidelines

This document defines how the built-in types and the standard library modules under `extension/` are designed. It governs API shape and conventions; the mechanics of wiring a module up (registration, docs, tests) are covered in `CLAUDE.md`.

It describes the target, not the present. The existing code predates it, so names and shapes cited here may not exist yet, and existing modules are never precedent: where code and this document disagree, the code has a bug to fix (raise it for review when the fix is breaking).

Three migrations are decided and in progress; write new code as if they were complete:

- **The `x/` tier is being removed.** There is one curated library. Do not add modules under `x/`, and do not move code there.
- **User-declared traits are being removed** from the language, together with trait dependencies, traits as values, `Trait.method(x)` calls and `value.traits()`. `impl` blocks remain, for the closed set of language protocols in §2.
- **The library defines no traits.** `io.Reader`, `io.Writer` and `json.ToJSON` give way to the method-name protocols in §2.

## 1. What the library is for

The standard library is small, high quality and built for the person writing the script.

- **Designed from tasks, not from packages.** Start from the programs people actually write (read a file, parse JSON, make a request, run a command, walk a directory, handle a time, match a pattern, read arguments), write the code you would want to write, and derive the API from that code. A member that appears in no realistic program is not added.
- **Documentation first.** Write the Goblin Book chapter before the implementation. Whatever reads awkwardly there is an API problem, and that is the cheapest moment to fix it.
- **A budget, not a backlog.** Smallness comes from refusing, not from pruning later. A new member has to say what it does better than a combination of existing ones; a new module has to say which task it makes possible.
- **Go is the substrate, not the model.** Modules are implemented on the Go standard library (or near-standard packages: `golang.org/x/*`, or de-facto standards with a stable API; anything else needs review). Take Go's *semantics* for edge cases — Unicode case mapping, path cleaning, URL escaping, number precision — because they are settled and the implementation sits on them anyway. Do not take Go's *names or shapes*: those were formed by constraints Goblin does not have (no default arguments, no exceptions, static types, `string`/`[]byte` twins, flag bitmasks). Familiarity from Go, POSIX or Python is not a reason to keep a name.
- **No second tier.** If something is not worth designing properly, it is not in the library. There is no mechanically wrapped layer to put it in.
- Every module is its own Go package at `extension/<module name>` exposing an `Execute` constructor, so a compiled program links only the modules it imports. Plumbing shared between modules lives under `extension/internal/`, never in a sibling module's package.

## 2. How types and the library fit together

There are exactly three mechanisms, and each has one job. A newcomer needs one sentence: **`impl` is for what the language's syntax does to your value; everything else is an ordinary method you call by name.**

### Language protocols: `impl`

A closed, built-in set. Each one connects a value to a piece of syntax or a built-in conversion:

| Protocol | Connects to |
|---|---|
| `Eq` | `==`, `!=` |
| `Ord` (needs `Eq`) | `<` `<=` `>` `>=`, sorting, `max`, `min`, `compare(a, b)` |
| `Hashable` (needs `Eq`) | dict keys, `hash(x)` |
| `Show` | `print`, conversion to String |
| `Truth` | conditions, `&&`, `||`, `!` |
| `Add` `Sub` `Mul` `Div` `Mod` `Neg` | arithmetic operators |
| `Iter` | `for x in value` |
| `Index` | `value[i]`, `value[i] = x` |

- The properties that make this the right mechanism for syntax are the ones to preserve: one required method derives a whole family that cannot disagree (`compare` gives every ordering operator and `sort`); dependencies are checked (`Ord` needs `Eq`); an empty `impl` of `Eq`/`Ord`/`Hashable`/`Show` is structural; protocol methods never enter the instance namespace, so a user method may be called `add` or `get` without changing what `+` or `[]` does; and a broken `impl` is reported when the type is defined.
- The set grows only with the language: a new protocol exists because new syntax needs it. The library cannot add one, and neither can user code.
- A protocol method is reached through its syntax. The two that have none are the global functions `compare(a, b)` and `hash(x)`.
- **Library value types implement the protocols that make sense for them**, and this is where "learn one type, guess the next" comes from: a time is comparable and subtractable, a path or a UUID is equal-comparable, hashable and printable. Prefer an operator the user already knows to a method they have to look up (`a < b`, not `a.before(b)`), but only where the meaning is the obvious one.

### Library protocols: a method with an agreed name

When a library function needs something from the object it is given, it asks for an ordinary method. There is one path, no trait alternative and nothing to import.

| Capability | The object has |
|---|---|
| readable stream | `read(size)` |
| writable stream | `write(data)` |
| releasable resource | `close()` |
| JSON-encodable | `to_json()` |

- This table is the complete list. Adding a row is an API decision that needs review, because the name becomes reserved in practice for every user type.
- A function that accepts one of these says so in its error: `csv.write_all() argument 'dest' must have a write(data) method, got Int`.
- Functions accept the widest sensible input rather than making the caller adapt it: `json.decode` takes a String, Bytes, or anything readable.
- **Reader shape.** `read(size)` with a non-negative Int returns a chunk as Bytes (consumers also tolerate String). End of stream is an empty chunk or `nil`: consumers accept both, producers return empty Bytes. A producer may also allow `read()` with no argument to mean "everything".
- **Writer shape.** `write(data)` takes a chunk (consumers pass Bytes; producers also accept String) and returns the number of bytes written; `nil` counts as the whole chunk, and consumers accept both.
- **Closing.** `close()` is optional on a stream, and a consumer never calls it: whoever opened the stream decides when it ends, normally with `defer`.
- Go-side code adapts these objects through `object.NewDuckReader` / `object.NewDuckWriter` rather than re-implementing the shapes. A new stream-shaped API follows these shapes exactly and documents the expected shape in the Goblin Book.

### The surface: methods on concrete types, and module functions

Everything a user calls is an ordinary method on a concrete type or a function in a module. Sharing between types is an implementation matter, done in Go (§6), never a user-visible abstraction.

## 3. Shape of a module

- **The common task is one call.** Reading a whole file, fetching a URL, running a command and getting its output should not require assembling objects first. The object-level API exists for the cases that need control.
- **A value type for the algebra of a name, module functions for I/O on what it names.** When a domain has a structured name (a path, a URL), operations on the name itself belong to a value type: components are properties, transformations are methods, and it performs no I/O (`p.parent`, `p.with_suffix(".md")`, `u.host`). Operations on the thing the name refers to are module functions that accept the name as a String or as the value type (`fs.read(path)`, `http.get(url)`), and functions that produce names return the value type. This keeps the first line of a script free of wrapping, because names mostly arrive as Strings (literals, arguments, configuration), and it gives one line to remember: `fs.` touches the disk, `Path` only looks at the name.
  - `fs` is the one module for files and paths: `Path` lives in it, there is no separate `path` module, and file-system locations (`cwd`, `home`, `temp_dir`) are `fs` functions rather than `os` ones.
  - A value type must not grow methods named like a library protocol of §2 unless it is that thing: a `Path` with a `write` method would be accepted wherever a writable stream is.
  - A parameter that already means data does not also accept a path. `json.decode(text)` and `csv.read_all(text)` take data or a readable object; reading the file is the caller's `fs.read(path)`.
- **An object when the task needs control.** The object-level API of a module is the thing that has state and a lifetime (`File`, `Command`, `Client`, `Response`), obtained from a module function and released with `close()` where it holds a resource.
- **Value-shaped domains stay as plain functions** (`math`, `base64`).
- **One home per operation.** The same operation is not offered as a global, a module function and a method at once. Pick the place a user would look first and delete the others. An operation that has syntax or a constructor has no method twin (`s * n`, not also `s.repeat(n)`; `a < b`, not also `a.compare(b)`); when an operator and a method would do the same job, keep one.
- **Lenient when harmless, explicit when destructive.** A non-destructive operation defaults to the forgiving behaviour (`fs.mkdir` creates missing parents and accepts an existing directory); a destructive one requires the dangerous part to be written out (`fs.remove(path, recursive=true)`).
- **The return type never depends on a boolean keyword.** Text and bytes are separate functions (`fs.read` returns a String, `fs.read_bytes` Bytes); where the input already carries the distinction, one function is enough (`fs.write` takes either).
- **Build with the language's own features.** These, not traits, are what make a Goblin API feel native:
  - *Keyword arguments with defaults* replace function families (below).
  - *`defer`* is the cleanup mechanism. Anything holding a resource has `close()`; the library adds no callback-scoped or `with`-style helpers.
  - *Iteration owns what it acquired.* One pass of iteration owns what it acquired when it started; what the iterable itself owns is not the iteration's to release. `for line in file` leaves the file open when the loop ends, while a value returned by something like `fs.read_lines(path)` opens the file when a pass starts (in its Go `Iterator`) and closes it in the pass's `Close`, which every consumer calls once however the pass ends (`break`, `return`, an error); iterating it again opens the file again.
  - *Destructuring* is how a function returns several things: `var head, tail = line.cut("=")`. Return a fixed-size list; do not return a status flag alongside a value when an error or `nil` says it better.
  - *First-class functions* are how behaviour is customized (`key=`, a predicate), not subclass-style hooks.
  - *Errors* signal failure (§7); there are no `(value, ok)` or sentinel-return conventions.
  - *Operators and `for`* via §2, for value types.

### One function, not a family

- The common case works with required positional arguments alone: `s.split(sep)`.
- Variations are optional keyword arguments whose defaults give the plain behaviour: `s.split(sep, count=-1)`, `gzip.compress(data, level=gzip.DEFAULT_COMPRESSION)`.
- Never expose `foo` and `foo_with_bar` side by side when a keyword argument can express the difference.
- Collapse configuration, not concepts. When two functions differ in *what* they do rather than in a knob, they stay separate: `url.query_escape` and `url.path_escape` apply different rules to different parts of a URL, and `escape(s, mode=...)` would trade two self-describing names for a stringly-typed enum.
- Keyword names are API. The same concept has the same name everywhere (`sep`, `count`, `chars`, `key`, `default`, `timeout`, `level`), and one name never means two things (`size` is a number of elements, so a channel's buffer is `capacity`). A boolean keyword names a state, not an action (`descending=true`, not `reverse=true`).

All argument handling goes through `object.ArgParser` (`object/argparse.go`): it defines positional order, keyword precedence, defaults and uniform `TypeError` messages. Do not hand-roll argument validation anywhere, the `object/` runtime included. For a parameter shape with no accessor yet, add the accessor.

## 4. Properties versus methods

A member that takes no arguments is either a property (`p.name`) or a nullary method (`p.exists()`). One rule decides, so the same kind of member looks the same on every type. What the reader takes from it: parentheses mean "this does something", no parentheses mean "this only looks".

- **Property** when all of the following hold: no arguments, no I/O, does not block, cannot fail, changes nothing. This covers components (`Path.parent`, `URL.host`, `Time.year`, `Match.span`), sizes (`size` on every collection), element access (`list.first`, `list.last`), predicates about the value itself (`Path.is_absolute`, `Result.success`), and current status, whoever drives it (`File.closed`, `Goblin.done`, `Command.running`).
- **A property never raises.** When the value can be absent for this receiver, the property is `nil` (`list.first` on an empty list). If absence needs an explanation rather than `nil`, the member is a method.
- **Method** when any of the following holds: it takes arguments; it performs I/O (`File.stat()`, `Response.json()`); it can block or fail; it mutates the receiver or the world; it returns a fresh collection the caller may modify (`dict.keys()`, `list.copy()`); it converts or transforms the value into a new one (`s.upper()`, `Path.to_posix()`); or it is expensive enough that the reader should see the cost.
- The same question may be a property in one place and a call in another, and the difference is information: `info.is_dir` reads a snapshot already taken, `fs.is_dir(path)` asks the disk.
- Module-level members follow the same rule. A fact fixed for the life of the process is a value (`os.pid`, `os.args`), a stateful facility is an object (`os.env`), and anything that can change or fail is a function.
- **A value's member namespace holds its fields and methods and nothing else.** Asking a value for its type or its member names is a global function taking the value, never a member, so no name is taken away from user types and the two backends have no shadowing order to agree on.

## 5. Naming

- **Case.** Functions, methods, properties and keywords are `snake_case`; types are `Capitalized`; module-level constants are `UPPER_CASE` with no exceptions for tradition (`math.PI`).
- **Underscores mark every word boundary**: `get_env`, `read_dir`, `temp_dir`, never `getenv`, `iterdir`, `tempdir`. The failure this prevents is unpredictability: beside `read_dir` and `has_prefix`, a run-together name has to be memorized on its own.
- **Abbreviations are fine when they are the word programmers actually use** (`env`, `dir`, `temp`, `pid`, `cwd`, `stat`, `args`); spelling them out helps nobody. Ones that are not self-evident get spelled out (`is_ipv4`, not `is4`; `modified_at`, not `mod_time`).
- **One spelling per concept**, across the whole library: `dir` and never `directory`, `temp` and never `tmp`, `remove` and never `del`. This includes type names: a type has one name, used by its constructor, by error messages and by the documentation.
- **One verb per concept, on every type.**

  | Concept | Name |
  |---|---|
  | membership test, returns Bool | `contains` |
  | position of a match | `index` |
  | the matching element itself | `find` |
  | delete by value or key | `remove` |
  | take out and return | `pop` |
  | read with a fallback | `get` |
  | whole data format ↔ value (`json`, `csv`, `base64`, text encodings) | `encode` / `decode` |
  | one value ↔ its text form (`time`, `url`) | `parse` / `format` |
  | stream and file I/O | `read` / `write`, `open` / `close` |
  | another representation of the same value | `to_x()`, always a method |

  A synonym from another language (`unlink`, `del`, `marshal`, `iterdir`, `as_x`) is a bug even where it is the familiar name. Converting between built-in types is the constructor's job (`Bytes(s)`), which by the one-home rule excludes a method doing the same.
- **A search that finds nothing returns `nil`**, never a sentinel such as `-1`. An operation that is expected to succeed raises instead (`pop` on a missing key without a default).
- **Booleans**: an adjective or participle stands alone (`closed`, `done`, `running`, `success`); a noun takes `is_` or `has_` (`is_dir`, `has_prefix`).
- **Mutation, on mutable types.** A method that changes the receiver is an imperative verb and returns the receiver (`push`, `sort`, `reverse`, `clear`); where a non-mutating counterpart exists it is the past participle (`sorted`, `reversed`). This rule does not apply to immutable types, whose methods always return new values under plain verbs (`upper`, `trim`, `replace`), nor to the established functional names (`map`, `filter`).
- **Index arguments mean one thing per type.** Whatever `x[i]` accepts (negative indices, bounds behaviour), slices and every method taking a position accept the same.
- **Name the meaning, not the implementation**: `Response.status` (Int) and `status_text`; `headers` for a collection; `URL.query`, not `raw_query`.
- **A facility with get/set/remove/list is an object, not four functions.** `os.env` behaves like a Dict (`os.env["HOME"]`, `os.env.get("HOME", "")`, `os.env.remove("X")`), reusing vocabulary the user already has.

## 6. Built-in types

The built-in types are the part of the library every program uses, so consistency between them matters most and is enforced mechanically.

- **A specification matrix, not per-type design.** Operations shared by List, String, Bytes and Dict (`contains`, `index`, `find`, `count`, `remove`, `join`, `split`, `replace`, `trim`, …) are specified once: same name, same argument order, same keyword names, same behaviour on a miss. A type lacks an operation only when it is meaningless there, not because nobody wrote it (`sum` without `max`/`min`, `start` on one type's `index` only, and `join` running in opposite directions on two types are the failures this prevents).
- **Shared in Go.** The matrix is implemented by common Go helpers that the types call into, and a conformance test walks operation × type and asserts signatures and miss behaviour agree. `object/callmethod.go` remains the single name-to-method mapping.
- **Frequent operations only.** `String` and `Bytes` are not a transliteration of Go's `strings`/`bytes`. Each keeps the operations that realistic programs use; variants are keyword arguments (splitting on whitespace is `split()` with no separator); the long tail (`index_any`, `contains_rune`, `equal_fold`, `split_after`, `to_valid_utf8`, …) is deleted rather than relocated. `Bytes` keeps what binary data needs; text work goes through `decode()`.
- **Aggregates have one home.** The global `max` and `min` are the only ones: several arguments or one iterable, an optional `key=`, a documented tie rule.

## 7. Errors

- Never return raw Go errors and never panic across the boundary. Wrap native errors with `object.WrapNativeError` / `object.WrapError`, attaching the appropriate kind from the hierarchy in `object/error.go` (`IOError`, `ParseError`, `PermissionError`, …).
- Add a new error kind only when callers realistically need to catch that case distinctly; otherwise use the nearest existing parent.
- Messages follow `funcname() <what failed>`. A bare `funcname() failed` says nothing: name the failure (`decode() invalid hex data`, `read() failed to read HTTP body`).
- A missing language protocol should say how to get it: `cannot compare Deck (add 'impl Ord' to type Deck)`.

## 8. Values, not Go internals

- Accept and return Goblin runtime types. Go machinery — channels, `context.Context`, config structs, interfaces as extension points — does not leak into a module's API.
- Where Go has `[]byte` and `string` variants, pick the one natural Goblin type and offer the other only if genuinely needed.
- APIs whose Go counterpart takes a `context.Context` are wrapped without a context parameter: pass `context.TODO()` internally and do not invent per-call timeout or cancellation arguments. Mirroring a timeout Go exposes as plain configuration is fine (`http.Client(timeout=...)`). Time-bounding belongs to the `Goblin` handle (`wait(timeout=...)`); cancellation, when it arrives, will ride on that handle, and per-call plumbing added today would only have to be unwound.

## 9. Leave it out

Some domains are too abstraction-heavy to offer simply (server-side HTTP, reflection, database drivers).

- Offer the simple, useful part and skip the rest, rather than shipping a distorted or half-faithful abstraction.
- Call out every deliberate omission in the PR description and, when a user would notice, in the module's Goblin Book chapter.
- A wrong abstraction shipped is worse than a missing one. With no second tier to fall back on, "not in the library" is the default answer.

## 10. Open language gaps

The closed protocol set means these can only be fixed in the language. Until they are, design around them rather than papering over them in a module.

- **User code cannot define a lazy sequence**: `for` pulls one item at a time and built-in sources such as files are lazy, but a user type's `iter` still has to return an existing iterable. A `next` language protocol with an end marker is the candidate fix.
- **No protocol for `size`**, so a user type cannot offer the property every built-in collection has.
- **No membership syntax**: `contains` is a method everywhere; if an `in` operator is added it needs a protocol.

## 11. Checklist for a new module

1. Write the programs it is for, then the Goblin Book chapter, before any Go (§1).
2. Justify the module and each member against the budget (§1).
3. Decide the shape: a one-call path for the common task, plain functions or a central type (§3).
4. Give its value types the language protocols that fit, and accept the library protocols of §2 instead of concrete types where a stream or resource is expected.
5. Decide property versus method for every argument-free member (§4).
6. Check every name and keyword against §5, and collapse families into keyword arguments (§3).
7. Implement with `ArgParser` and the error hierarchy (§7), on the Go standard library (§1).
8. Register it, add `examples/` and tests (see `CLAUDE.md`).
9. List what was left out, for review (§9).
