# os

The os module is the running process: its arguments, environment, standard
streams and exit status. Files are in [fs](./module-fs.md) and other programs
in [exec](./module-exec.md).

## Common tasks

~~~goblin
import "os"

# Positional arguments
if os.args.size != 2 {
    eprint("usage: {os.program} <source> <target>")
    os.exit(2)
}
var source, target = os.args

# Settings from the environment
var port = Int(os.env.get("PORT", default="8080"))
var token = os.env["API_TOKEN"]          # KeyError if it is not set
os.env["NO_COLOR"] = "1"                 # seen by programs started later

# A filter: stdin to stdout
for line in os.stdin {
    if line.contains("ERROR") {
        print(line)
    }
}

# Ask a question: the built-in input() writes the prompt and reads a line
var answer = input("Delete everything? [y/N] ")
if answer == nil || answer.trim().lower() != "y" {
    os.exit(1)
}

# Platform differences
var opener = "xdg-open"
if os.platform == "darwin" {
    opener = "open"
}
~~~

`docs/examples/os-tasks.goblin` runs these tasks with arguments and piped
input; `examples/os.goblin` covers the rest of the module.

## Module members

| Member | Kind | Value |
| --- | --- | --- |
| `args` | List of String | The program's arguments, **without** the program name. `goblin run app.goblin a b` and `./app a b` both give `["a", "b"]`; `[]` in the REPL. It is built once: every read gives the same List |
| `program` | String | What the program was started as: the source path under `goblin run`, the executable path when compiled, `""` in the REPL |
| `env` | Env | The process environment, live; see [Env](#env) |
| `stdin` | `fs.File` | Readable: `for line in os.stdin`, `read_line()`, `read()` |
| `stdout` | `fs.File` | Writable; `write(data)` adds no newline |
| `stderr` | `fs.File` | Writable |
| `pid` | Int | The process id |
| `platform` | String | `"linux"`, `"darwin"`, `"windows"`, … (Go's `runtime.GOOS`) |
| `exit(code=0)` | function | Ends the process with that status, from 0 to 255 (ValueError otherwise) |

A fact fixed for the life of the process is a value (`os.args`, `os.pid`),
the environment is an object, and ending the process is a function.

`os.exit()` ends the process at once: deferred calls, including those of the
calling function, do not run, so a temporary directory removed by a `defer`
stays on disk. Clean up before calling it. An interrupt (Ctrl-C) also ends the
process without running deferred calls. Avoid `os.exit()` in library code;
raise an error instead and let the program decide.

The standard streams are ordinary [File](./module-fs.md#file) values whose
`path` is `nil`; their methods raise IOError when the stream fails. `print`
writes to the same stdout and neither is buffered, so their output interleaves
in call order. `close()` on a standard stream is allowed and closes the
process's stream.

## Env

`os.env` is one object instead of four functions. It is not a Dict, but it uses
Dict's vocabulary and nothing else. Names and values are Strings; anything else
is a TypeError. Changes affect this process and the programs it starts
afterwards.

| Member | Value | When the variable is missing |
| --- | --- | --- |
| `env[name]` | The variable's value | KeyError `environment variable 'X' is not set` |
| `env[name] = value` | Sets it; ValueError for an empty name or one containing `=` or NUL | — |
| `get(name, default=nil)` | The value, or `default` | `default` |
| `contains(name)` | Bool; a variable set to `""` is contained | `false` |
| `remove(name)` | Unsets it and returns the Env | fine |
| `to_dict()` | A new Dict snapshot of every variable, sorted by name | — |

Env is not iterable and has no `size`, `keys()` or `items()`: use `to_dict()`
when the whole environment is wanted. The module exports the type name `Env`
for `type_of(os.env) == os.Env`; it cannot be called.

## Left out

- Prompting is the built-in `input(prompt)`, not a member of this module. It
  reads through the same buffer as `os.stdin`, so the two can be mixed.
- Options, help and usage errors are the [cli](./module-cli.md) module's;
  `os.args` is the raw list.
- The host name and user and group ids are not in the module:
  `exec.capture(["hostname"]).stdout.trim()` or `exec.capture(["id", "-u"])`.
- The working directory and temporary directories are in fs: `fs.cwd()`,
  `fs.mkdir_temp()`.
