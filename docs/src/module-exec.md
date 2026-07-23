# exec

The `exec` module follows Go's `os/exec` package. Commands run directly without
a shell, so arguments are not expanded as globs, pipelines, redirects, or
environment-variable references.

~~~goblin
import "exec"

var output = exec.Command("git", ["status", "--short"]).output()
print(output.decode())
~~~

## Module API

| Function | Go equivalent |
| --- | --- |
| `Command(name, args=[], dir=unit, env=unit, stdin=unit)` | `exec.Command` plus `Cmd` field setup |
| `look_path(file)` | `exec.LookPath` |

`args` is a list of strings. Goblin keyword arguments configure the
Go `Cmd.Dir`, `Cmd.Env`, and `Cmd.Stdin` fields while the command is created.
`dir` accepts a string or `Path`. `env` accepts a dictionary of string keys and
values; `unit` inherits the current environment. `stdin` accepts `Str`, `Bytes`,
or `unit`.

## Cmd

| Method | Go equivalent |
| --- | --- |
| `run()` | `Cmd.Run` |
| `start()` | `Cmd.Start` |
| `wait()` | `Cmd.Wait` |
| `output()` | `Cmd.Output` |
| `combined_output()` | `Cmd.CombinedOutput` |

`run`, `start`, and `wait` return `unit` on success. The output methods return
`Bytes`, since process output is not necessarily UTF-8. As in Go, a non-zero
exit status is an error for `run`, `wait`, `output`, and `combined_output`;
Goblin exposes it as an `IOError`.

Call either `run`, `output`, or `combined_output` for one-step execution. For
explicit asynchronous control, call `start` followed by `wait`. A `Cmd`
represents one command execution and must not be reused.

## Shell commands

Pass each argument as a separate list element. If shell syntax is explicitly
required, invoke a platform shell yourself, for example
`exec.Command("sh", ["-c", script])`. Never interpolate untrusted text into a
shell script.
