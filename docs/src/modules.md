# Modules and imports

A module groups related values. Import statements belong at module scope. An
imported name is the final component of its path.

~~~goblin
import "json"
import "./modules/greeter"

print(json.marshal({"ok": true}))
greeter.greet("world")
~~~

## Local modules

Local paths normally start with ./ or ../ and are resolved relative to the file
that imports them. Omit the .goblin suffix. A local module chooses its public
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

The remaining chapters document each built-in module separately.
