# Your first program

Create `hello.goblin`:

```goblin
print("Hello, world!")
```

Goblin does not require a `main` function. Statements at the top level run in
order. Add a comment with `#` when you need to explain a line:

```goblin
# A program may have more than one top-level statement.
var audience = "world"
print("Hello,", audience)
```

## Run it directly

```sh
$ goblin run hello.goblin
Hello, world!
```

`run` parses and interprets the file. It is the usual command during development.
If parsing, semantic checking, or execution fails, the command reports the
source location and exits with an error.

## Compile an executable

```sh
$ goblin build-exe hello.goblin
$ ./hello
Hello, world!
```

By default the output name is derived from the source file. Specify a path with
`-o` when needed:

```sh
$ goblin build-exe -o bin/hello hello.goblin
```

`build-exe` requires a working Go toolchain because it invokes `go build`
after generating Go source. The output path may be absolute or relative to the
current directory.

## Use the REPL

Goblin also includes an interactive REPL:

```sh
$ goblin repl
>>> print("Hello, world!")
Hello, world!
```

The REPL saves history. Its prompt changes to `...` while you enter a
multi-line function or block. Press Ctrl-D to exit.

Use the REPL for small experiments. Definitions stay available for the rest of
the session:

```text
>>> var answer = 42
>>> answer + 1
43
```
