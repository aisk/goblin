# Troubleshooting

When a program fails, first identify which stage reported the problem. The
message and traceback include the source location.

| Symptom | Likely cause | What to check |
| --- | --- | --- |
| Parse error | Syntax the grammar does not accept | Braces, commas, string escapes, and function parameter syntax |
| Semantic error | A name, declaration, or statement is invalid in its scope | Declare names before use; keep import/type/export at module scope |
| NameError / AttributeError | A runtime name or member is unavailable | Spelling, module import, and `value.attributes()` in the REPL |
| TypeError | An operation received an unsupported value type | Constructor inputs, callback return values, and operator operands |
| IndexError / KeyError | A list index or dictionary key is absent | Bounds checks, dict.get(), and list.index() returning -1 |
| IOError / NetworkError | A filesystem or HTTP operation failed | Paths, permissions, connectivity, and a try/catch recovery boundary |

## A reliable debugging loop

Reduce the failing code to a small `.goblin` file, then run it through the
interpreter:

~~~sh
goblin run failing.goblin
~~~

Use `goblin repl` for inspecting values. `value.attributes()` lists the
operations an object exposes, which is particularly useful for module values,
responses, files, and custom types.

If `build-exe` fails after a program works with `run`, verify that the Go
toolchain is installed and run the command again with the generated-build
message visible. Report the Goblin source, command, full error text, and
whether the interpreter path succeeds when filing an issue.

`build-exe` compiles against the published Goblin runtime unless it finds a
local source checkout (from the working directory or the executable's
location). When developing Goblin itself, set `GOBLIN_ROOT` to the checkout
path so compiled programs use your local runtime.
