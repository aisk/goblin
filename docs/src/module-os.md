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
~~~

| Function | Purpose |
| --- | --- |
| getenv(key) / setenv(key, value) / unsetenv(key) | Read or change environment values |
| environ() | Return all environment values as a dictionary |
| getwd() / hostname() | Current directory and machine name |
| getpid() / getppid() | Process identifiers |
| tempdir(dir="", pattern="") / tempfile(dir="", pattern="") | Create temporary paths |
| exit(code=0) | End the process |

Avoid using exit() inside reusable library code. Environment and temporary-file
operations can raise IOError.

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

tempdir() and tempfile() accept optional directory and pattern arguments and
return created paths. They are helpful for generated output and tests.

~~~goblin
var directory = os.tempdir("", "goblin-")
var filename = os.tempfile(directory, "data-")
print(directory)
print(filename)
~~~

getpid(), getppid(), getuid(), getgid(), and getgroups() expose identity
information supplied by the host operating system. Availability and exact
values can vary by platform.
