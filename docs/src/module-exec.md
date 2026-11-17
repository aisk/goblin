# exec

The exec module runs other programs. Three functions cover three tasks: `run`
for a command's effect, `capture` for its output, `start` for a process you
keep control of.

A command is always a **List**: the program followed by its arguments. No
shell is involved, so there is no quoting and no injection; for a pipeline,
run `["sh", "-c", "a | b"]` on purpose.

## Common tasks

~~~goblin
import "exec"
import "fs"
import "http"
import "os"
import "time"

# Run a step; the program stops with an error if it fails
exec.run(["go", "build", "./..."])
exec.run(["npm", "install"], cwd="web", env={"CI": "1"})

# Get a command's output
var head = exec.capture(["git", "rev-parse", "HEAD"]).stdout.trim()
var tags = exec.capture(["git", "tag"]).stdout.lines()

# Feed it input
var sorted_text = exec.capture(["sort", "-u"], stdin="b\na\nb\n").stdout

# A failure you expect: look at the result instead of catching
var diff = exec.run(["git", "diff", "--quiet"], check=false)
if !diff.success {
    print("working tree is dirty (status {diff.code})")
}

# A failure you want to report your own way
try {
    exec.capture(["git", "push"])
} catch e: exec.ExitError {
    eprint("push failed with status {e.code}")
    eprint(e.stderr)
    os.exit(1)
}

# Is the tool there at all
if exec.find("docker") == nil {
    raise Error("docker is not installed")
}

# Send output to a file
var log = fs.open("build.log", write=true)
defer log.close()
exec.run(["make"], stdout=log, stderr=log)

# Give up after a minute
exec.run(["npm", "test"], timeout=60)

# Start a server, use it, stop it
var server = exec.start(["python3", "-m", "http.server", "8000"])
defer server.kill()
time.sleep(1)
print(http.get("http://localhost:8000/").status)

# Follow a long-running command line by line
var tail = exec.start(["tail", "-f", "app.log"], stdout=exec.PIPE)
for line in tail.stdout {
    if line.contains("READY") {
        break
    }
}
tail.kill()

# Show a build's output as it comes, and stop if it fails
var build = exec.start(["make"], stdout=exec.PIPE)
for line in build.stdout {
    print("make: {line}")
}
build.wait()                 # raises exec.ExitError for a non-zero exit

# Read binary output as Bytes
var archive = exec.start(["tar", "-cz", "src"], stdout=exec.PIPE)
var data = archive.stdout.read()
archive.wait()
~~~

`examples/exec.goblin` runs each of these tasks with small shell commands.

## Module members

| Member | Returns | Behaviour |
| --- | --- | --- |
| `run(args, cwd=nil, env=nil, stdin=nil, stdout=nil, stderr=nil, check=true, timeout=nil)` | Result | Runs the command to completion; its output goes where this program's goes, or to `stdout=`/`stderr=`. `result.stdout` and `stderr` are `nil` |
| `capture(args, cwd=nil, env=nil, stdin=nil, check=true, timeout=nil)` | Result | Runs the command to completion, collecting stdout and stderr as text in the Result. Nothing reaches the terminal. Output that is not valid UTF-8 raises ValueError: read binary output through `start` with `stdout=exec.PIPE` |
| `start(args, cwd=nil, env=nil, stdin=nil, stdout=nil, stderr=nil)` | Command | Starts the command and returns at once. `stdin`, `stdout` and `stderr` also accept `exec.PIPE` |
| `find(name)` | Path or `nil` | Where `name` would be found on `PATH`; `nil` when it is not installed |
| `ExitError` | error kind | Raised by `run`, `capture` and `Command.wait` for a non-zero exit under `check=true` |
| `PIPE` | constant | For `start` only: connect that stream to this program (`command.stdin`, `command.stdout`, `command.stderr`). Passing it to `run` is a TypeError |

The parameters they share:

| Parameter | Accepts | Meaning |
| --- | --- | --- |
| `args` | non-empty List of String or Path | Program and arguments. A bare name is looked up on `PATH` |
| `cwd=nil` | String, Path | Working directory for the command; `nil` is this program's |
| `env=nil` | Dict of String to String or `nil` | **Added to** the inherited environment; a `nil` value removes that variable |
| `stdin=nil` | `nil`, String, Bytes or a readable | `nil`: the command reads this program's stdin. Data is fed to the command, then its stdin is closed |
| `stdout=nil`, `stderr=nil` | `nil` or a writable | `nil`: the command writes to this program's stream. A writable (a File, anything with `write(data)`) receives the output and is not closed |
| `check=true` | Bool | A non-zero exit raises. `false` returns the Result for you to inspect |
| `timeout=nil` | seconds | When it expires the process is killed and TimeoutError is raised |

The type names `Command`, `Result` and `Pipe` (the type of `PIPE`) are exported
for `type_of`; calling them raises TypeError pointing at `exec.start` /
`exec.run`.

### Errors

| Failure | Raises | Data |
| --- | --- | --- |
| The program is not found | NotExistError `run() program "gti" not found` | — |
| `cwd` is missing or not a directory | NotExistError, IOError | `path`: the `cwd` given, a Path |
| It cannot be run | PermissionError, IOError | — |
| `timeout` expired | TimeoutError `run() "sleep" timed out after 2 seconds` | `stdout`, `stderr`: what `capture` had collected when the process was killed; `nil` from `run` |
| Non-zero exit with `check=true` | `exec.ExitError` `run() "git" exited with status 128`; `capture` adds the last non-empty stderr line: `capture() "git" exited with status 128: fatal: not a git repository`; `Command.wait` says `wait() "make" exited with status 2` | `code`, `stdout`, `stderr` |
| `capture` collected output that is not valid UTF-8 | ValueError `capture() "cat" wrote stdout that is not valid UTF-8 (use exec.start with stdout=exec.PIPE for bytes)`, after the timeout and exit checks | — |

The data of an `exec.ExitError` is what the Result would have shown under
`check=false`: `code` is the exit status (`-1` when the process was killed by
a signal, never `0`), `stdout` and `stderr` are the captured text from
`capture` and `nil` from `run` and `Command.wait`. It is not an IOError: the
command ran, and its answer was "failed". The output an ExitError or a
TimeoutError carries is for diagnosis, so invalid UTF-8 in it is replaced by
U+FFFD rather than raising a second error.

Pass `check=false` when a non-zero status is an answer rather than a failure;
catch `exec.ExitError` when it is a failure you want to handle.

## Result

Immutable; all properties.

| Property | Value |
| --- | --- |
| `code` | Int exit status; `-1` when the process was killed by a signal |
| `success` | Bool, `code == 0` |
| `stdout` | String from `capture`, always valid UTF-8; `nil` from `run` and `Command.wait` |
| `stderr` | likewise |

## Command

A started process, from `exec.start`. It shows as `<Command ["git", "status"]>`.

| Member | Kind | Value |
| --- | --- | --- |
| `pid` | property | Int |
| `running` | property | Bool: started and not yet exited |
| `stdin` | property | A writable [File](./module-fs.md#file) when started with `stdin=exec.PIPE`, else `nil`. `close()` it to signal the end of input |
| `stdout`, `stderr` | property | A readable File when started with `exec.PIPE`, else `nil` |
| `wait(timeout=nil, check=true)` | method | Blocks until the process exits and returns its Result (`stdout`/`stderr` are `nil`). A non-zero exit raises `exec.ExitError` as `run` does; `check=false` returns the Result instead. Calling it again gives the same outcome. On timeout it raises TimeoutError and the process **keeps running** |
| `kill()` | method | Stops the process; `nil`. Fine if it already exited. A killed process has status `-1`, so collect it with `wait(check=false)` |

A piped `stdout` or `stderr` must be read to its end before `wait()`: a
process blocked on a full pipe never exits. Close the pipes you asked for once
you are done with them.

When `stdout=` or `stderr=` is a writable, its `write` is called while the
command runs, also while your program goes on after `start`; do not use the
same object from Goblin code until `wait()` returns.

## Left out

Shell command strings, signals other than kill, a clean (not inherited)
environment, and capturing output as Bytes in one call are not part of the
module. Binary output is read from `exec.start(args, stdout=exec.PIPE)` with
`command.stdout.read()`, or sent to a File with `stdout=`.
