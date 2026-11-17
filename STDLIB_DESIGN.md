# Goblin Standard Library Design Guidelines

This document defines how the built-in types and the standard library modules under `extension/` are designed. It governs API shape and conventions; the mechanics of wiring a module up (registration, docs, tests) are covered in `CLAUDE.md`.

The library has been rebuilt to follow it. Where code and this document disagree, one of them has a bug: fix the code, or fix the document if the rule was wrong. An existing module is not a precedent on its own; the rule it follows is. Goblin keeps no backward compatibility, so a change that breaks existing scripts is made when the rule calls for it, and the scripts in the repository are updated with it.

Neither the language nor the library has a way to declare an interface. `impl` names one of the closed set of language protocols in §2, and the library asks for ordinary methods by name.

## 1. What the library is for

The standard library is small, high quality and built for the person writing the script.

- **Start from tasks.** List the programs people actually write (read a file, parse JSON, make a request, run a command, walk a directory, handle a time, match a pattern, read arguments), write the code you would want to write for each, and derive the API from that code. Do not add a member that appears in no realistic program.
- **Review the task list before the members.** A module with few members is fine when its tasks are covered: `json` has two functions because its task has two directions. A task a user cannot finish is a design defect: logging in and then fetching a page, reading a `.gz` log larger than memory, showing a time in another zone. For every module, ask which ordinary script cannot be completed with it. If there is one, add the task to the list and then the members it needs.
- **Write the Goblin Book chapter before the implementation.** Anything that reads awkwardly in the chapter is an API problem, and it is cheapest to fix at that point.
- **Justify every addition.** A new member has to do something better than a combination of existing members. A new module has to make some task possible.
- **Implement on Go, but do not copy its API.** Modules are implemented on the Go standard library (or near-standard packages: `golang.org/x/*`, or de-facto standards with a stable API; anything else needs review). Use Go's behaviour for edge cases such as Unicode case mapping, path cleaning, URL escaping and number precision, because it is settled and the implementation depends on it anyway. Do not use Go's names or shapes: they come from constraints Goblin does not have (no default arguments, no exceptions, static types, `string`/`[]byte` twins, flag bitmasks). A name being familiar from Go, POSIX or Python is not a reason to keep it.
- **No module wraps a Go package function by function.** Each member is designed by the rules of this document, and a member that is not worth that design is not added.
- Every module is its own Go package at `extension/<module name>` exposing an `Execute` constructor, so a compiled program links only the modules it imports. Plumbing shared between modules lives under `extension/internal/`, never in a sibling module's package.

## 2. How types and the library fit together

There are three mechanisms. `impl` covers what the language's syntax does to a value; everything else is an ordinary method called by name.

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
- **Library value types implement the protocols that make sense for them**, so an operator a user knows from one type works the same way on the next: a time is comparable and subtractable, a path or a URL is equal-comparable, hashable and printable. Prefer an operator the user already knows to a method they have to look up (`a < b`, not `a.before(b)`), but only where the meaning is the obvious one.

### Library protocols: a method with an agreed name

When a library function needs something from the object it is given, it asks for an ordinary method. There is one path and nothing to import.

| Capability | The object has |
|---|---|
| readable stream | `read(size)` |
| writable stream | `write(data)` |
| JSON-encodable | `to_json()` |

- This table is the complete list. Adding a row is an API decision that needs review, because the name becomes reserved in practice for every user type.
- `close()` is not in the table. An object that holds a resource has a `close()` method by convention, and no library function calls it on a value it was given.
- A function that accepts one of these says so in its error: `run() argument 'stdout' must be Nil or have a write(data) method, got Int`.
- Functions accept the widest sensible input rather than making the caller adapt it: `json.decode` takes a String, Bytes, or anything readable.
- **Reader shape.** `read(size)` with a non-negative Int returns a chunk as Bytes (consumers also tolerate String). End of stream is an empty chunk or `nil`: consumers accept both, producers return empty Bytes. A producer may also allow `read()` with no argument to mean "everything".
- **Writer shape.** `write(data)` takes a chunk (consumers pass Bytes; producers also accept String) and returns the number of bytes written; `nil` counts as the whole chunk, and consumers accept both.
- **Closing.** A stream need not have `close()`, and a consumer never calls it: whoever opened the stream decides when it ends, normally with `defer`.
- Go-side code adapts these objects through `object.NewDuckReader` / `object.NewDuckWriter` rather than re-implementing the shapes. A new stream-shaped API follows these shapes exactly and documents the expected shape in the Goblin Book.

### The surface: methods on concrete types, and module functions

Everything a user calls is an ordinary method on a concrete type or a function in a module. Sharing between types is an implementation matter, done in Go (§6), never a user-visible abstraction.

## 3. Shape of a module

- **The common task is one call.** Reading a whole file, fetching a URL, running a command and getting its output should not require assembling objects first. The object-level API exists for the cases that need control.
- **A value type for the algebra of a name, module functions for I/O on what it names.** When a domain has a structured name (a path, a URL), operations on the name itself belong to a value type: components are properties, transformations are methods, and it performs no I/O (`p.parent`, `p.with_suffix(".md")`, `u.host`). Operations on the thing the name refers to are module functions that accept the name as a String or as the value type (`fs.read(path)`, `http.get(url)`), and functions that produce names return the value type. Names mostly arrive as Strings (literals, arguments, configuration), so accepting both means a script does not wrap them first. The split also tells the reader where I/O happens: the functions of `fs` do I/O, and the members of `Path` do not.
  - `fs` is the one module for files and paths: `Path` lives in it, there is no separate `path` module, and file-system locations (`cwd`, `home`, `temp_dir`) are `fs` functions rather than `os` ones.
  - A value type must not grow methods named like a library protocol of §2 unless it is that thing: a `Path` with a `write` method would be accepted wherever a writable stream is.
  - A parameter that already means data does not also accept a path. `json.decode(data)` and `csv.decode(data)` take data or a readable object; reading the file is the caller's `fs.read(path)`.
- **When unsure, provide the lower-level piece.** The user composes it. Leave a convenience out when it does work the caller cannot see (opening and closing a file named by a String inside an upload), decides a policy for them (whether and which requests to retry), or carries a rule that has to be memorized (resolving a request path against a base URL). The explicit form is a few more lines the reader can follow: `fs.open` plus `defer`, a retry loop with `catch` and `time.sleep`, an interpolated URL. A convenience added later changes no existing script, so leaving one out until a task shows it is needed costs little.
  - This rule does not justify leaving a task unfinishable. If the library has to keep some state for a task to work at all, that state belongs to the object: a `Client` remembers cookies, because otherwise "log in, then request" means parsing and replaying headers by hand.
  - The one-call functions of the first rule are the deliberate exception, and their names say what they do: `fs.read(path)` opens, reads and closes.
- **Do not add a type that only holds arguments.** A type needs an algebra (a path, a URL), or state and a lifetime (a file, a client, a response). A set of options for one call is keyword arguments, and when it has to travel as a value it is a Dict expanded with `**`: `http.request(**options)`. A client-side `Request` object would only give that Dict a second spelling.
- **An object when the task needs control.** The object-level API of a module is the thing that has state and a lifetime (`File`, `Command`, `Client`, `Response`), obtained from a module function and released with `close()` where it holds a resource.
- **Value-shaped domains stay as plain functions** (`math`, `base64`).
- **Each operation is offered in one place.** The same operation is not offered as a global, a module function and a method at once. Pick the place a user would look first and delete the others. An operation that has syntax or a constructor has no method twin (`s * n`, not also `s.repeat(n)`; `a < b`, not also `a.compare(b)`); when an operator and a method would do the same job, keep one.
- **How far an operation reaches must be visible in the call.** `fs.mkdir` creates missing parents and accepts an existing directory, because creating loses nothing. Removing a directory also removes everything below it, which the path alone does not show, so it needs `fs.remove(path, recursive=true)`. Functions that write or move to a target path (`fs.write`, `fs.copy`, `fs.move`) replace a file already there, because the call names that file.
- **Raise by default where the alternative is a silently wrong result.** When an operation completes but its outcome says the task failed (a command exits non-zero, a server answers 404), the default is to raise an error carrying the facts (§7). The same keyword everywhere, `check=false`, hands the outcome back for the caller to inspect: `exec.run`, `exec.capture` and `Command.wait` all take it. Without it `http.get(url).json()` parses an error page and `fs.write(path, http.get(url))` saves one as the download.
- **Names that may repeat.** Where a name usually has one value but may have several (HTTP headers, query parameters), the collection is an ordinary Dict with one value per name, and one more member returns every value of a name as a List: `response.header_values("set-cookie")`, `url.query_values(text, "tag")`. Reading always returns one shape, never "a String, or a List when the name repeats". On the sending side a List value sends the name once per element.
- **The return type never depends on a boolean keyword.** Text and bytes are separate functions (`fs.read` returns a String, `fs.read_bytes` Bytes); where the input already carries the distinction, one function is enough (`fs.write` takes either, or a readable stream, which is what makes "download to a file" one call).
- **Build with the language's own features.** These, not declared interfaces, are what make a Goblin API feel native:
  - *Keyword arguments with defaults* replace function families (below).
  - *`defer`* is the cleanup mechanism. Anything holding a resource has `close()`; the library adds no callback-scoped or `with`-style helpers.
  - *A pass of iteration releases what it opened, and nothing else.* A resource that a pass opens when it starts is closed when that pass ends; a resource the iterable already held is not closed by the pass. `for line in file` leaves the file open when the loop ends, while a value returned by something like `fs.read_lines(path)` opens the file when a pass starts (in its Go `Iterator`) and closes it in the pass's `Close`, which every consumer calls once however the pass ends (`break`, `return`, an error); iterating it again opens the file again.
  - *Destructuring* is how a function returns several things: `var head, tail = line.cut("=")`. Return a fixed-size list; do not return a status flag alongside a value when an error or `nil` says it better.
  - *First-class functions* are how behaviour is customized (`key=`, a predicate), not subclass-style hooks.
  - *Errors* signal failure (§7); there are no `(value, ok)` or sentinel-return conventions.
  - *Operators and `for`* via §2, for value types.

### One function, not a family

- The common case works with required positional arguments alone: `s.split(sep)`.
- Variations are optional keyword arguments whose defaults give the plain behaviour: `s.split(sep, count=nil)`, `gzip.compress(data, level=nil)`.
- An absent limit is `nil`, never a sentinel such as `-1`: `count=nil` means "no limit", and a negative `count` raises ValueError.
- Never expose `foo` and `foo_with_bar` side by side when a keyword argument can express the difference.
- Merge functions that differ in a setting; keep functions that differ in what they do. When two functions differ in *what* they do rather than in a knob, they stay separate: `url.query_escape` and `url.path_escape` apply different rules to different parts of a URL, and `escape(s, mode=...)` would trade two self-describing names for a stringly-typed enum.
- Keyword names are API. The same concept has the same name everywhere (`sep`, `count`, `chars`, `key`, `default`, `timeout`, `level`), and one name never means two things (`size` is a number of elements, so a channel's buffer is `capacity`). A boolean keyword names a state, not an action (`descending=true`, not `reverse=true`).

All argument handling goes through `object.ArgParser` (`object/argparse.go`): it defines positional order, keyword precedence, defaults and uniform `TypeError` messages. Do not hand-roll argument validation anywhere, the `object/` runtime included. For a parameter shape with no accessor yet, add the accessor.

## 4. Properties versus methods

A member that takes no arguments is either a property (`p.name`) or a nullary method (`file.close()`). One rule decides, so the same kind of member looks the same on every type, and a reader can tell from the parentheses whether a member may do work: a property only reads the value, a method may do I/O, fail or change something.

- **Property** when all of the following hold: no arguments, no I/O, does not block, cannot fail, changes nothing. This covers components (`Path.parent`, `URL.host`, `Time.year`, `Match.start`), sizes (`size` on every collection), element access (`list.first`, `list.last`), predicates about the value itself (`Path.is_absolute`, `Result.success`), and current status, whoever drives it (`File.closed`, `Goblin.done`, `Command.running`).
- **A property never raises.** When the value can be absent for this receiver, the property is `nil` (`list.first` on an empty list). If absence needs an explanation rather than `nil`, the member is a method.
- **Method** when any of the following holds: it takes arguments; it performs I/O (`File.read_line()`, `Response.json()`); it can block or fail; it mutates the receiver or the world; it returns a fresh collection the caller may modify (`dict.keys()`, `list.copy()`); it converts or transforms the value into a new one (`s.upper()`, `Path.with_suffix(".md")`); or it is expensive enough that the reader should see the cost.
- The same question may be a property in one place and a call in another, and the difference is information: `info.is_dir` reads a snapshot already taken, `fs.is_dir(path)` asks the disk.
- Module-level members follow the same rule. A fact fixed for the life of the process is a value (`os.pid`, `os.args`, which holds the arguments without the program name so `var src, dst = os.args` works; the name is `os.program`), a stateful facility is an object (`os.env`), and anything that can change or fail is a function.
- **A value's member namespace holds its fields and methods and nothing else.** Asking a value for its type or its member names is a global function taking the value, never a member, so no name is taken away from user types and the two backends have no shadowing order to agree on.

## 5. Naming

- **Case.** Functions, methods, properties and keywords are `snake_case`; types are `Capitalized`; module-level constants are `UPPER_CASE` with no exceptions for tradition (`math.PI`).
- **Underscores mark every word boundary**: `get_env`, `read_dir`, `temp_dir`, never `getenv`, `iterdir`, `tempdir`. The failure this prevents is unpredictability: beside `read_dir` and `has_prefix`, a run-together name has to be memorized on its own.
- **Abbreviations are fine when they are the word programmers actually use** (`env`, `dir`, `temp`, `pid`, `cwd`, `stat`, `args`); spelling them out helps nobody. Ones that are not self-evident get spelled out (`is_ipv4`, not `is4`; `modified_at`, not `mod_time`).
- **A module name is one lower-case word.** It must not be the name users naturally give a variable next to it (`cli`, not `args`: `var args = …` would collide with the import). Keeping an established name is acceptable even when it hides a global function in the importing file (`import "hash"` hides `hash(x)` there); the module's chapter says so.
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
  | whole data format ↔ value (`json`, `csv`, `base64`; Bytes to text) | `encode` / `decode` |
  | one value ↔ its text form (`time`, `url`) | `parse` / `format` |
  | stream and file I/O | `read` / `write`, `open` / `close` |
  | another representation of the same value | `to_x()`, always a method |

  A synonym from another language (`unlink`, `del`, `marshal`, `iterdir`, `as_x`) is a bug even where it is the familiar name.
- **Conversions between built-in types.** Converting a value to a built-in type is that type's constructor (`Bytes(s)`, `Int(x)`, `List(xs)`), and no method does the same. `String(x)` is the one exception to how far that goes: it always gives the display text, the text `print` shows, so `String(bytes)` is `b"…"`. Reading Bytes as UTF-8 text is therefore the method `bytes.decode()`, which raises ValueError on invalid UTF-8.
- **A search that finds nothing returns `nil`**, never a sentinel such as `-1`. An operation that is expected to succeed raises instead (`pop` on a missing key without a default).
- **Booleans**: an adjective or participle stands alone (`closed`, `done`, `running`, `success`); a noun takes `is_` or `has_` (`is_dir`, `has_prefix`).
- **Mutation, on mutable types.** A method that changes the receiver is an imperative verb and returns the receiver (`push`, `sort`, `reverse`, `clear`). There are no non-mutating counterparts; `xs.copy().sort()` is the sorted copy. If one is ever added, it is named with the past participle (`sorted`). This rule does not apply to immutable types, whose methods always return new values under plain verbs (`upper`, `trim`, `replace`), nor to the established functional names (`map`, `filter`).
- **Positions follow one rule on every sequence type.** A position names either an element (`x[i]`, `pop(index)`) or a boundary between elements (`insert(index, value)`, `index(value, start=)`, the ends of a slice), and a negative position counts from the end on List, String and Bytes alike. An element or a boundary outside the sequence raises IndexError, because it names something that is not there. A slice is the exception: it takes the elements that exist between its ends, so an end beyond the sequence stops at its edge and a low end past the high one gives an empty result. Scripts take "the first ten" (`xs[:10]`) or "the last three lines" (`lines[-3:]`) without knowing the size, and the shorter result is the correct answer for a shorter input. An index has no such answer: there is no element to return. All of this is resolved in `object/position.go`.
- **Names say what a value means to the user**, not how it is stored or computed: `Response.status` (Int) and `status_text`; `headers` for a collection; `URL.query`, not `raw_query`.
- **A facility with get/set/remove/list is an object, not four functions.** `os.env` uses the vocabulary of a Dict: indexing (`os.env["HOME"]`, `os.env["X"] = "1"`), `get`, `contains` and `remove`, plus `to_dict()` for a snapshot. It is not a Dict: it reads the live environment, so it has no `size` and is not iterable.

## 6. Built-in types

The built-in types are the part of the library every program uses, so consistency between them matters most and is enforced mechanically.

- **Shared operations are specified once, in a matrix.** Operations shared by List, String, Bytes and Dict are specified once: same name, same argument order, same keyword names, same behaviour on a miss. The tables in `object/matrix_test.go` are that specification: which type has which member (`contains`, `count` and `index` on the three sequences, `split`, `replace` and `cut` on String and Bytes, `trim` on String only, `find` and `join` on List only, …), and what each returns on a miss or an out-of-range position. A type lacks an operation only when it is meaningless there, not because nobody wrote it (`sum` without `max`/`min`, `start` on one type's `index` only, and `join` running in opposite directions on two types are the failures this prevents).
- **Shared in Go.** The matrix is implemented by common Go helpers that the types call into, and a conformance test (`object/matrix_test.go`) walks operation × type and asserts members, keyword names, miss behaviour and receiver returns agree; a new member of a built-in type goes into its tables first. `object/callmethod.go` remains the single name-to-method mapping.
- **Only operations that realistic programs use.** `String` and `Bytes` are not a transliteration of Go's `strings`/`bytes`. Each keeps the operations that realistic programs use; variants are keyword arguments (splitting on whitespace is `split()` with no separator); the long tail (`index_any`, `contains_rune`, `equal_fold`, `split_after`, `to_valid_utf8`, …) is deleted rather than relocated. `Bytes` keeps what binary data needs; text work goes through `decode()`.
- **Aggregates are global functions only.** The global `max` and `min` are the only ones: several arguments or one iterable, an optional `key=`, a documented tie rule.

## 7. Errors

- Never return raw Go errors and never panic across the boundary. Wrap native errors with `object.WrapNativeError` / `object.WrapError`, attaching the appropriate kind from the hierarchy in `object/error.go` (`IOError`, `ParseError`, `PermissionError`, …).
- Add a new error kind only when callers realistically need to catch that case distinctly (`catch e: exec.ExitError`); otherwise use the nearest existing parent. A module's kind is made with `object.NewErrorKind(name, parent)` and exported as a module value, not a global; errors of a kind are made with `object.NewKindError(kind, format, ...)`, whose message stands alone, the way Goblin code writes `Kind("message")`.
- Facts a caller acts on travel as data on the error, not inside the message: `object.NewKindError(...).With("code", status)` or `object.WrapNativeError(object.IOError, msg, err).With("path", p)` on the freshly built error, read in Goblin as `e.code`. Data is seen through every later `wrap`; never attach it to an error that may already have been raised. Data names may not be `traceback`, `cause`, `wrap` or `is`.
- Messages follow `funcname() <what failed>`. A bare `funcname() failed` says nothing: name the failure (`decode() invalid Base64 at position 5`, `read() failed to read HTTP body`).
- An error raised by syntax rather than by a call (indexing, iteration, an operator) has no function to name, so its message starts with what failed: `environment variable 'X' is not set`, `line 3 of "x" is not valid UTF-8`, `cannot add String and Int`.
- A missing language protocol should say how to get it: `cannot compare Deck (add 'impl Ord' to type Deck)`.

## 8. Values, not Go internals

- Accept and return Goblin runtime types. Go machinery — channels, `context.Context`, config structs, interfaces as extension points — does not leak into a module's API.
- Where Go has `[]byte` and `string` variants, pick the one natural Goblin type and offer the other only if genuinely needed.
- APIs whose Go counterpart takes a `context.Context` are wrapped without a context parameter; a context, if the implementation needs one, stays internal. A blocking operation is time-bounded the way the language bounds every blocking operation: a `timeout=` keyword read through `ArgParser.Timeout` (a number of seconds, `nil` for no limit) that raises TimeoutError (`exec.run(cmd, timeout=60)`, `Chan.receive(timeout=...)`, `Goblin.wait(timeout=...)`). Never expose a context, a deadline object or a cancellation argument.

## 9. What to leave out

Some domains are too abstraction-heavy to offer simply (server-side HTTP, reflection, database drivers).

- Offer the simple, useful part and skip the rest, rather than shipping a distorted or half-faithful abstraction.
- Call out every deliberate omission in the PR description and, when a user would notice, in the module's Goblin Book chapter.
- **Leaving something out must not close the door on it.** Check that today's names and shapes leave room for the omitted part: the HTTP client keeps the name `Request` free and its header, query and body conventions symmetrical, so that a server can later share one `Request` type with it instead of forcing a redesign.
- When it is unclear whether a convenience is worth adding, do not add it: it can be added later without changing any script. When it is unclear whether a task can be completed, provide the lower-level piece (§3), because a task that cannot be finished is a defect.

## 10. Open language gaps

The closed protocol set means these can only be fixed in the language. Until they are, design around them rather than papering over them in a module.

- **User code cannot define a lazy sequence**: `for` pulls one item at a time and built-in sources such as files are lazy, but a user type's `iter` still has to return an existing iterable. A `next` language protocol with an end marker is the candidate fix.
- **No protocol for `size`**, so a user type cannot offer the property every built-in collection has.
- **No membership syntax**: `contains` is a method everywhere; if an `in` operator is added it needs a protocol.
- **No bitwise operators**: the permission bits of `FileInfo.mode` and `fs.chmod` can only be tested or combined with arithmetic (`mode / 64 % 2 == 1` for "owner may execute"). Operators on Int are the fix; a module should not add bit functions in the meantime.

Closed by the language, so no module should offer its own version:

- **Formatting numbers and padding text**: a format specifier in an interpolation (`"{x:>8.2f}"`) and the global `format(value, spec)` for a spec built at run time. A module formats only its own values' text forms (`time` layouts); there is no number formatting in `math` or elsewhere.
- **Errors that carry data and are caught by kind**: `catch e: Kind`, `Kind("message", **data)`, and `object.NewErrorKind` / `With` in Go (§7).
- **Reading a line from standard input**: the global `input(prompt="")`, nil at the end of input. It reads through `object.Stdin`, the buffer `os.stdin` shares; a module that reads standard input must go through it too.
- **Code point and character**: the String property `code_point` and the Int method `to_char()`.
- **Numbers in other bases**: `format(n, "x")` (also `"o"`, `"b"`) writes one, `Int(text, base=16)` reads one. Hex text from Bytes is `to_hex()`; the reverse is built from `Int(..., base=16)` rather than added as a member: `Bytes(range(s.size / 2).map(func(i) { return Int(s[i * 2:i * 2 + 2], base=16) }))`.

## 11. Checklist for a new module

1. Write the programs it is for, then the Goblin Book chapter, before any Go (§1).
2. Justify the module and each member against the budget (§1).
3. Decide the shape: a one-call path for the common task, plain functions or a central type (§3). Re-read the task list and ask which ordinary script still cannot be completed (§1); then strike every convenience that hides work or decides a policy (§3).
4. Give its value types the language protocols that fit, and accept the library protocols of §2 instead of concrete types where a stream or resource is expected.
5. Decide property versus method for every argument-free member (§4).
6. Check every name and keyword against §5, and collapse families into keyword arguments (§3).
7. Implement with `ArgParser` and the error hierarchy (§7), on the Go standard library (§1).
8. Register it, add `examples/` and tests (see `CLAUDE.md`).
9. List what was left out, for review (§9).
