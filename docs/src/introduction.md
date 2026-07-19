# Introduction

**Goblin** is a toy programming language built for fun. It is dynamically
typed, with a syntax that borrows from Go and Python.

Goblin programs can be executed in two ways:

- **Interpreted** — `goblin run` executes a source file directly with a
  tree-walking interpreter. There is also an interactive REPL (`goblin repl`).
- **Compiled** — `goblin build-exe` transpiles the source to Go and compiles
  it into a native executable.

A quick taste:

```goblin
func greet(name) {
    print("Hello,", name)
}

for name in ["world", "goblin"] {
    greet(name)
}
```

The source code is available on [GitHub](https://github.com/aisk/goblin).
