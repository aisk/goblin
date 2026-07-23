# os

The os module provides process and environment information. Use it for
configuration supplied by the operating system rather than hard-coding secrets
or deployment settings.

~~~goblin
import "os"

var port = os.getenv("PORT")
if port == "" {
    port = "8080"
}
print(port)
print(os.getwd())
print(os.hostname())

# argv() is the program command line (index 0 is the invocation name)
for arg in os.argv() {
    print(arg)
}
~~~

| Function | Purpose |
| --- | --- |
| argv() | Return the program command-line arguments as a list of strings |
| getenv(key) / setenv(key, value) / unsetenv(key) | Read or change environment values |
| environ() | Return all environment values as a dictionary |
| getwd() / hostname() | Current directory and machine name |
| getpid() / getppid() | Process identifiers |
| mkdir_temp(dir="", pattern="") | Create a temporary directory |
| create_temp(dir="", pattern="") | Create and open a temporary file |
| exit(code=0) | End the process |

Avoid using exit() inside reusable library code. Environment and temporary-file
operations can raise IOError. argv() does not accept arguments and returns a
fresh list each call; mutating that list does not change the program arguments.

## Command-line arguments

argv() presents the command line from the Goblin program's point of view. Index
0 is its invocation name; remaining elements are the arguments passed after it.
With `goblin run`, the source path is the invocation name — for example `goblin
run app.goblin foo bar` makes `argv()` return `["app.goblin", "foo", "bar"]`.
Arguments that look like flags (such as `-v` or `--help`) are forwarded only
when they appear *after* the source file. Put the source file first. Leading
flags are rejected.
For CLI help use `goblin run -h` or `goblin help run` (alone, with no source
file). Compiled executables from `build-exe` see the real process argv (the
binary path at index 0). In the REPL, `argv()` is `[""]` so interactive
sessions do not expose the goblin process arguments. Use argv() when a program
needs flags or positional inputs from the shell; prefer getenv() for
configuration that should not be visible on the command line.

~~~goblin
var args = os.argv()
if args.size() < 2 {
    print("usage:", args[0], "<file>")
    os.exit(1)
}
print("input:", args[1])
~~~

## Configuration from the environment

getenv() returns an empty string for a missing key. Use that behavior to supply
a development default, or use environ() when a program needs to inspect the
complete environment dictionary.

~~~goblin
var env = os.environ()
var mode = env.get("APP_MODE", default="development")
var debug = mode == "development"
print(debug)
~~~

setenv() changes only the environment of the current Goblin process and
processes it starts. It does not persist after the program exits.

## Temporary paths and process identity

mkdir_temp() and create_temp() mirror Go's os.MkdirTemp and os.CreateTemp.
Both accept optional directory and pattern arguments, positionally or by name.
mkdir_temp() returns the created path; create_temp() returns an open fs.File
whose name attribute contains its path.

~~~goblin
var directory = os.mkdir_temp(pattern="goblin-")
var file = os.create_temp(directory, pattern="data-")
print(directory)
print(file.name)
file.close()
~~~

getpid(), getppid(), getuid(), getgid(), and getgroups() expose identity
information supplied by the host operating system. Availability and exact
values can vary by platform.
