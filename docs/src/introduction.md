# Introduction

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
