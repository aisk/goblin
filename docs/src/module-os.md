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
