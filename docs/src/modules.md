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

Names without export remain private. Imports, function definitions, and type
definitions are resolved before the module's remaining top-level statements
run, so an exported function can refer to a definition later in the file.

The remaining chapters document each built-in module separately.
