# Your first program

Create `hello.goblin`:

```goblin
print("Hello, world!")
```

## Run it directly

```sh
$ goblin run hello.goblin
Hello, world!
```

`run` parses and interprets the file. It is the usual command during development.

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

## Use the REPL

Goblin also includes an interactive REPL:

```sh
$ goblin repl
>>> print("Hello, world!")
Hello, world!
```

The REPL saves history. Its prompt changes to `...` while you enter a
multi-line function or block. Press Ctrl-D to exit.
