# Modules and imports

A module groups related values. Import statements belong at module scope. An
imported name is the final component of its path.

~~~goblin
import "json"
import "./modules/greeter"

print(json.encode({"ok": true}))
greeter.greet("world")
~~~

## Local modules

Local paths must start with ./ or ../ and are resolved relative to the file
that imports them; any other path names a standard-library module (like
"json"). Omit the .goblin suffix. A local module chooses its public
names with export.

~~~goblin
# modules/greeter.goblin
var greeting = "Hello"

func greet(name) {
    print(greeting, name)
}

export greet
~~~

Names without export remain private. Define a module-level name before code
that uses it; this also applies to names referenced from an exported function.
Named functions may call themselves recursively.

An exported name is a live binding, not a copy: reading `counter.count` gives
the module's variable `count` as it is at that moment, including assignments
the module makes after its `export` statement or later from its own
functions. Only the module itself can assign its variables; `counter.count =
10` from outside raises TypeError (`Module does not support attribute
assignment`).

~~~goblin
# counter.goblin
var count = 0

func bump() {
    count = count + 1
    return count
}

export count
export bump
~~~

~~~goblin
# main.goblin
import "./counter"

print(counter.bump(), counter.count, counter.bump(), counter.count)   # 1 1 2 2
~~~

## A complete local module

For a small project, keep the entry point and local module in separate files:

~~~text
project/
├── main.goblin
└── modules/
    └── greeter.goblin
~~~

~~~goblin
# modules/greeter.goblin
var greeting = "Hello"

func greet(name) {
    return greeting + ", " + name
}

export greet
~~~

~~~goblin
# main.goblin
import "./modules/greeter"

print(greeter.greet("world"))
~~~

Run the entry point from `project/` with `goblin run main.goblin`. Local import
paths are resolved from the importing file, not from the shell's current
directory. A local import's final path component becomes its module name, so
`./modules/greeter` is available as `greeter`.

A module is loaded once and cached; later imports of the same file reuse the
loaded module, so every module that imports it sees the same variables. Two modules importing each other is a cycle, reported as
`circular import detected`.

The remaining chapters document each built-in module separately.
