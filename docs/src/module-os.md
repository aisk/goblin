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

# argv() is the process command line (index 0 is the program name)
for arg in os.argv() {
    print(arg)
}
~~~

| Function | Purpose |
| --- | --- |
| argv() | Return the process command-line arguments as a list of strings |
| getenv(key) / setenv(key, value) / unsetenv(key) | Read or change environment values |
| environ() | Return all environment values as a dictionary |
| getwd() / hostname() | Current directory and machine name |
| getpid() / getppid() | Process identifiers |
| tempdir([dir, pattern]) / tempfile([dir, pattern]) | Create temporary paths |
| exit(code=0) | End the process |

Avoid using exit() inside reusable library code. Environment and temporary-file
operations can raise IOError. argv() does not accept arguments and returns a
fresh list each call; mutating that list does not change the process arguments.

## Command-line arguments

argv() mirrors Go's os.Args. Index 0 is the program name or invocation path;
remaining elements are the arguments passed after it. With `goblin run`, pass
script arguments after the source file — for example `goblin run app.goblin foo
bar` makes `argv()` return `["app.goblin", "foo", "bar"]`. Arguments that look
like flags (such as `-v` or `--help`) are forwarded only when they appear
*after* the source file. Put the source file first. Leading flags are rejected.
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

tempdir() and tempfile() accept optional directory and pattern arguments in
that positional order, and return created paths. They are helpful for generated
output and tests. These functions do not accept named arguments.

~~~goblin
var directory = os.tempdir("", "goblin-")
var filename = os.tempfile(directory, "data-")
print(directory)
print(filename)
~~~

getpid(), getppid(), getuid(), getgid(), and getgroups() expose identity
information supplied by the host operating system. Availability and exact
values can vary by platform.
