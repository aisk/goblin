# Hello, World!

Create a file named `hello.goblin`:

```goblin
print("Hello, world!")
```

## Run it with the interpreter

```sh
$ goblin run hello.goblin
Hello, world!
```

## Compile it to a native executable

```sh
$ goblin build-exe hello.goblin
$ ./hello
Hello, world!
```

## Try the REPL

Goblin also ships with an interactive REPL:

```sh
$ goblin repl
>>> print("Hello, world!")
Hello, world!
```
