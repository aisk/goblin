# Built-in modules

A module groups values in a source file or exposes a built-in integration. Use
import at module scope; the imported name is the final component of the import
path.

~~~goblin
import "json"
import "./modules/greeter"

var value = json.unmarshal("{\"ok\": true}")
greeter.greet("world")
~~~

## Local modules

Local import paths normally start with ./ or ../ and are resolved relative to
the file containing the import. Omit the .goblin suffix.

~~~goblin
import "./modules/greeter"
~~~

The file modules/greeter.goblin chooses its public interface with export. Names
without export remain private to that module.

~~~goblin
# modules/greeter.goblin
var greeting = "Hello"

func greet(name) {
    print(greeting, name)
}

export greeting
export greet
~~~

Imports, function definitions, and type definitions are resolved before a
module's remaining top-level statements run. This lets an exported function
refer to another definition that appears later in its source file.

## Built-in modules

Goblin includes the following modules. Import only the modules you use.

| Module | Typical members | Use case |
| --- | --- | --- |
| json | marshal, unmarshal | JSON encoding and decoding |
| fs | read, write, exists, stat, read_dir | Files and directories |
| os | getenv, setenv, getwd, hostname, getpid | Process and environment |
| path | cwd, home | Current and home directory paths |
| time | now, sleep, parse, unix, since | Time and durations |
| random | int, intn, float, choice | Random values |
| math | pi, abs, sqrt, pow, floor, ceil | Mathematical operations |
| http | get, post, put, delete | HTTP client requests |
| mime | type_by_extension, extensions_by_type | MIME type lookup |

### JSON

json.marshal converts a Goblin value to JSON text. json.unmarshal parses JSON
text and returns Goblin values.

~~~goblin
import "json"

var encoded = json.marshal({"name": "Ada", "active": true})
var decoded = json.unmarshal(encoded)
print(decoded["name"])
~~~

### Files and paths

The fs module handles simple file operations; path exposes the current and home
directories.

~~~goblin
import "fs"
import "path"

var filename = "notes.txt"
fs.write(filename, "remember this")
print(fs.read(filename))
print(fs.exists(filename))
print(path.cwd())
fs.remove(filename)
~~~

### Time, random values, and math

~~~goblin
import "math"
import "random"
import "time"

print(math.sqrt(81))
print(random.intn(10))
print(time.now())
~~~

Module functions can raise errors, for example when a file is missing, JSON is
invalid, or an HTTP request fails. Handle these with [Errors](./errors.md).
