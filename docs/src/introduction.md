# Introduction

![Goblin logo](https://repository-images.githubusercontent.com/650676396/467e8cc2-8df2-445a-a477-b7bb399394a2)

Goblin is a dynamically typed language with Go-style braces and a compact,
Python-like feel. It is useful for experimenting with programs and for
learning how interpreters, runtimes, and code generation fit together.

This book assumes that you can use a command line and already know the basic
ideas of variables, functions, and control flow. Its aim is not to catalogue
every API, but to get you comfortably reading and writing Goblin programs.

Goblin has two ways to run a program:

- `goblin run` interprets a source file directly. Use it while developing and
  experimenting.
- `goblin build-exe` transpiles source to Go and compiles a native executable.
  Use it when you want a standalone program.

Both paths parse the same language and run semantic checks before executing.
During development, start with `goblin run`; use `build-exe` when you need an
executable without the Goblin CLI. The generated executable still uses the
Goblin runtime, so the language behavior is intended to be the same.

Here is a complete Goblin program:

```goblin
func greet(name) {
    print("Hello,", name)
}

for name in ["world", "Goblin"] {
    greet(name)
}
```

Source files use the `.goblin` extension. Comments start with `#`. When you
are ready, continue with [Installation](./installation.md).

## What this book covers

The chapters first cover values and control flow, then functions and
collections, followed by custom types, modules, and errors. The examples are
complete fragments that can be pasted into a `.goblin` file. The Book focuses
on the language and its standard runtime; see the repository's `examples/`
directory for larger executable programs.
