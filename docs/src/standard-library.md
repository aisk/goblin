# Standard library

Goblin's standard library provides modules for common program boundaries:
files, environment variables, paths, time, data formats, networking, random
values, mathematics, and MIME metadata. Import a module at module scope, then
access its members with dot notation.

~~~goblin
import "json"
import "path"

var cwd = path.cwd()
var text = json.marshal({"cwd": cwd})
print(text)
~~~

The standard library is separate from Goblin's built-in functions and types.
For example, print(), range(), Int(), List(), Dict(), and Chan() are available
without an import. Import a module only when its capabilities are needed.

## Module map

| Module | Main purpose | Start with |
| --- | --- | --- |
| [json](./module-json.md) | Encode and decode JSON | marshal(), unmarshal() |
| [fs](./module-fs.md) | Read, write, inspect, and remove files | read(), write(), exists() |
| [os](./module-os.md) | Read environment and process information | getenv(), getwd(), hostname() |
| [path](./module-path.md) | Find the current or home directory | cwd(), home() |
| [time](./module-time.md) | Work with time and durations | now(), sleep(), parse() |
| [random](./module-random.md) | Generate random values or choose an item | intn(), float(), choice() |
| [math](./module-math.md) | Numeric constants and functions | pi, sqrt(), pow(), abs() |
| [http](./module-http.md) | Make HTTP requests | get(), post(), put() |
| [mime](./module-mime.md) | Look up MIME types and extensions | type_by_extension() |

## Imports and errors

Built-in module names are simple strings such as "json" and "fs". Local source
modules use a relative import such as "./modules/greeter"; those are documented
in [Modules and imports](./modules.md) because they use the same import syntax.

Most standard-library operations that touch the outside world can fail. JSON
parsing may raise ParseError, a missing file may raise an I/O-related error, and
HTTP requests may fail. Use try/catch around work that your program can recover
from; see [Errors](./errors.md).

## Choosing a module

Use json whenever a program boundary expects JSON rather than trying to build
JSON text manually. Prefer fs for simple whole-file reads and writes, and use
its open() function when a file object is needed. Use path.cwd() or path.home()
instead of assuming a current directory. Use time.sleep() only for intentional
delays, and use Chan plus spawn() for communication between concurrent Goblin
functions.

Each module has its own chapter in this section, with a focused API reference
and example.
