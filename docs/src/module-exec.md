# exec

Import `exec` to configure and execute external commands. Commands are invoked
directly: arguments are never parsed by a shell.

~~~goblin
import "exec"

var cmd = exec.Command(
    "git",
    ["status", "--short"],
    stdout=exec.CAPTURE,
    stderr=exec.CAPTURE
)

var result = cmd.run()
if result.success {
    print(result.stdout.decode())
}
~~~

## Command

~~~text
Command(name, args=[], cwd=unit, env=unit,
        stdin=INHERIT, stdout=INHERIT, stderr=INHERIT)
~~~

`name` and every element of `args` must be strings. `cwd` accepts `unit`, a
string, or a `Path`. An omitted `env` inherits the current process environment;
a dictionary replaces it completely, so `env={}` starts the command with an
empty environment. Environment keys and values must be strings.

The standard-stream policies are:

| Policy | Meaning |
| --- | --- |
| `INHERIT` | Use the corresponding Goblin process stream |
| `DISCARD` | Provide EOF for stdin or discard output |
| `CAPTURE` | Capture stdout or stderr into the result |

`stdin` also accepts `Str` or `Bytes`. `CAPTURE` is valid only for stdout and
stderr. Captured values are `Bytes`, because command output is not necessarily
UTF-8; call `decode()` when text is expected.

## Executing a command

`cmd.run()` starts, waits for, and reaps a command. Output behavior comes only
from the stream configuration on `Command`.

~~~goblin
var result = exec.Command(
    "gofmt",
    stdin="package main\nfunc main(){}",
    stdout=exec.CAPTURE,
    stderr=exec.CAPTURE
).run()
~~~

For explicit asynchronous control, use `start()` followed by `wait()`:

~~~goblin
var cmd = exec.Command("worker", ["--once"])
cmd.start()
print(cmd.pid)
var result = cmd.wait()
~~~

A command can be started only once. `wait()` before `start()` and a second
execution attempt raise `ValueError`. Repeated calls to `wait()` return the
same cached result. `kill()` terminates a started command; call `wait()`
afterward to obtain its result. `cmd.pid` is `unit` before startup, and
`cmd.running` reports whether the command has not yet been reaped.

## Result

| Attribute | Type | Meaning |
| --- | --- | --- |
| `code` | `Int` | Exit code; a signal may produce `-1` |
| `success` | `Bool` | Whether the exit code is zero |
| `stdout` | `Bytes` or `unit` | Captured stdout, if configured |
| `stderr` | `Bytes` or `unit` | Captured stderr, if configured |

A non-zero exit code is a normal result, not an exception. Failures to start
or wait for the command raise an I/O-related error. Inspect `code` or `success`
and implement any command-specific failure policy in Goblin code.

## Shell commands

`exec` does not interpret pipes, redirects, glob patterns, or shell variables.
Pass every argument as a separate list element. If shell syntax is explicitly
required, invoke a platform shell yourself, for example `exec.Command("sh",
["-c", script])`; do not insert untrusted text into such a script.
