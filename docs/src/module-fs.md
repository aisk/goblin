# fs

The fs module works with files, directories and path names. Its functions
read and change the disk; the members of a `Path` work on the name alone and
never touch the disk.

Every parameter called `path`, `src` or `dst` accepts a String or a `Path`, and
every function that produces a file name returns a `Path`. Text is UTF-8.

## Common tasks

~~~goblin
import "fs"
import "time"

# Read and write whole files
var text = fs.read("notes.txt")
fs.write("copy.txt", text)
fs.write("events.log", "started\n", append=true)

# A big file, one line at a time; the file is closed when the loop ends
var errors = 0
for line in fs.read_lines("server.log") {
    if line.contains("ERROR") {
        errors = errors + 1
    }
}

# Build and take apart names (no disk access)
var source = fs.Path("docs/guide.md")
print(source.name, source.stem, source.suffix)   # guide.md guide .md
var target = fs.Path("build").join(source.with_suffix(".html").name)
print(target)                                    # build/guide.html

# Make sure the output directory is there, then write into it
fs.mkdir(target.parent)
fs.write(target, render(fs.read(source)))

# List, match, walk
for entry in fs.read_dir(".") {
    print(entry.name, fs.stat(entry).size)
}
for page in fs.glob("docs/**/*.md") {
    print(page)
}
func hidden(entry) {
    return entry.name.has_prefix(".")
}
for file in fs.walk(".", skip=hidden) {
    print(file.relative_to(fs.cwd()))
}

# A scratch directory for this run
var scratch = fs.mkdir_temp(prefix="report-")
defer fs.remove(scratch, recursive=true)
fs.write(scratch.join("data.json"), "[]")

# Write a file piece by piece
var out = fs.open("big.csv", write=true)
defer out.close()
for row in rows {
    out.write("{row}\n")
}

# Copy, move, delete, permissions
fs.copy("config.toml", "config.toml.bak")
fs.move("draft.md", "posts/final.md")
fs.remove("old.log")
fs.chmod("deploy.sh", 0o755)

# Is it there, how old is it
if fs.exists("cache.json") {
    var age = time.now() - fs.stat("cache.json").modified_at
}
~~~

`examples/fs.goblin` in the repository runs each of these tasks against a
scratch directory.

## Module functions

| Function | Returns | Behaviour |
| --- | --- | --- |
| `Path(text)` | Path | A Path from a String or Path; see [Path](#path) |
| `read(path)` | String | The whole file as text; an empty file is `""`. ValueError if it is not valid UTF-8 |
| `read_bytes(path)` | Bytes | The whole file as bytes |
| `read_lines(path)` | iterable of String | Lazy; see [Lines](#reading-lines) |
| `write(path, data, append=false)` | `nil` | Creates the file (mode `0o644`) or replaces its content; `append=true` adds to the end. `data` is a String, Bytes or a readable, which is streamed. Parent directories are not created. The file is emptied before a readable is read, so when reading it fails partway the file holds what was written up to then |
| `open(path, write=false, append=false)` | File | Read-only by default, and the file must exist. `write=true`: write-only, created or truncated. `append=true`: write-only, created, writes go to the end, also when `write=true` is given too |
| `exists(path)` | Bool | Whether the path names anything. Follows symlinks, so a broken link is `false`. A directory on the way that cannot be searched raises PermissionError rather than answering |
| `is_file(path)` / `is_dir(path)` | Bool | A regular file / a directory, after following symlinks. Missing is `false`; PermissionError as for `exists` |
| `stat(path)` | FileInfo | A snapshot of the entry; follows symlinks, and describes a broken link itself |
| `read_dir(path)` | List of Path | The entries, each joined onto `path`, sorted by name. An empty directory is `[]` |
| `walk(path, skip=nil)` | iterable of Path | Lazy. Every file below `path`, depth first, names sorted within each directory |
| `glob(pattern)` | List of Path | Paths matching `pattern` (a String or a Path, such as `dir.join("*.txt")`), sorted; no match is `[]` |
| `mkdir(path)` | `nil` | Creates the directory and any missing parents (mode `0o755`). An existing directory is fine |
| `mkdir_temp(prefix="")` | Path | A new, uniquely named directory under `temp_dir()`; the caller removes it |
| `remove(path, recursive=false)` | `nil` | Removes a file, a symlink (not its target) or an empty directory; a non-empty directory needs `recursive=true`. Removing what is already absent succeeds |
| `copy(src, dst)` | `nil` | `dst` is the new path itself. A file is copied with its permission bits, replacing an existing file; a directory is copied as a tree, `dst` must not exist and must not be inside `src`. A symbolic link given as `src` is followed, so its target's content is copied (a broken one raises NotExistError); links inside a copied tree are copied as links |
| `move(src, dst)` | `nil` | Renames; `dst` is the new path itself and an existing file there is replaced. Across file systems it copies, then removes; a symbolic link is moved as a link there too |
| `chmod(path, mode)` | `nil` | Sets permission bits, an Int from `0` to `0o777` such as `0o755` |
| `resolve(path)` | Path | The absolute form: joined onto `cwd()`, cleaned, symlinks resolved. A path that does not exist is still made absolute |
| `cwd()` / `home()` / `temp_dir()` | Path | The working directory, the user's home directory, the directory for temporary files |

The module also exports the type names `File`, `FileInfo`, `Lines` and `Walk`
so `type_of(v) == fs.File` works. They cannot be called:
`fs.File()` raises `TypeError: File cannot be constructed; use fs.open(path)`.

### Walking and matching

`walk` yields files only: directories are descended, not yielded, and symbolic
links are yielded but never followed. `skip` is a function called with the
Path of every entry, file or directory; when it returns true the entry is not
yielded and, for a directory, not entered. Pruning has to be a predicate
because a loop body cannot tell the walk what not to enter.

In a `glob` pattern, `*` and `?` match within one name, `[a-z]` is a character
class, and `**` as a whole component matches zero or more directories. A
pattern matches files and directories, and names starting with a dot like any
other. A relative pattern is relative to the working directory. A malformed
pattern raises ValueError.

A directory that cannot be read stops `walk` and `glob` with PermissionError
for the whole call, not only for that directory. `walk` can avoid one by name
with `skip`: `fs.walk(root, skip=func(p) { return p.name == "private" })`.

### Errors

Failures keep their kind: NotExistError, ExistError, PermissionError, or
IOError for the rest. The message names the function and the path, and the
operating system's error is the `cause`:

~~~goblin
try {
    fs.read("nope.txt")
} catch e: NotExistError {
    print(e.path)                # nope.txt
    print(e.message)             # read() cannot open "nope.txt": no such file or directory
}
~~~

An error about a path carries it as `e.path`, a Path; a line that is not valid
UTF-8 also carries `e.line`.

| Error | Data |
| --- | --- |
| NotExistError, ExistError, PermissionError, IOError from any `fs` function | `path`: the Path the message quotes, as given. For `copy` and `move` it is whichever of `src` and `dst` the failure is about |
| ValueError `read() "x" is not valid UTF-8` | `path` |
| ValueError `line 3 of "x" is not valid UTF-8` (`read_lines`, a File opened from a path) | `path`, and `line`, an Int counted from 1 |
| IOError from a File's `read`, `write` or `close` | `path` when the File has one; none for standard streams and pipes |
| TypeError, and ValueError about an argument (`mode`, `prefix`, a malformed `glob` pattern, a closed File) | none: they are mistakes in the call, not facts about the disk |

A handler further up can use the data without parsing the message:

~~~goblin
func load_all(paths) {
    return paths.map(func(p) { return fs.read(p) })
}

try {
    var texts = load_all(os.args)
} catch e: NotExistError {
    eprint("missing: {e.path}")
} catch e: PermissionError {
    eprint("cannot read {e.path.name}; check its permissions")
}
~~~

`fs.remove(dir)` on a non-empty directory raises IOError
`remove() directory "dir" is not empty (pass recursive=true)`; `mkdir` raises
ExistError when a file that is not a directory is in the way; `copy` and `move`
raise ExistError when `dst` is a directory (or, when copying a directory, when
`dst` exists at all). Copying a directory to a path inside itself, such as
`fs.copy(".", "dist/pkg")`, raises IOError
`copy() cannot copy "." into itself ("dist/pkg")` with `e.path` set to `dst`,
before anything is written.

## Path

An immutable name. It performs no I/O.

- `fs.Path(text)` takes exactly one String or Path. The text is cleaned:
  repeated separators and `.` are dropped, `..` is applied lexically, and `""`
  becomes `.`.
- Paths compare (`==`, `<`), hash and print by their text, with the platform's
  separator. A Path never equals a String: compare `String(p) == "a/b"` or
  `p == fs.Path("a/b")`.
- Inside a List, a Dict or a structural Show, a Path shows as `Path("a b")`,
  its text quoted like a String element, so it is not mistaken for a String:
  `print([fs.Path("a b"), "a b"])` prints `[Path("a b"), "a b"]`. `print(p)`,
  `String(p)` and `"{p}"` give the plain text.
- There is no `/` operator; joining is `join`.

| Member | Kind | Value |
| --- | --- | --- |
| `name` | property | The last component (`"report.tar.gz"`); `""` for a root and for `.` |
| `stem` | property | `name` without its suffix (`"report.tar"`) |
| `suffix` | property | From the last dot of `name` (`".gz"`); `""` when there is none, and for a name whose only dot is the first character (`.bashrc`) |
| `parent` | property | The path without its last component. A root is its own parent; the parent of `"a"` is `.` |
| `is_absolute` | property | Bool |
| `join(*segments)` | method | The path with each segment (String or Path) appended, then cleaned. `Path("a").join("/b")` is `a/b` |
| `with_name(name)` | method | The last component replaced. ValueError for an empty name, one containing a separator, or a path with no name |
| `with_suffix(suffix)` | method | The suffix replaced; `""` removes it. ValueError unless `suffix` is `""` or starts with `.` |
| `relative_to(base)` | method | The path leading from `base` to this one, worked out from the names alone; may start with `..`. ValueError when one is absolute and the other is not |

## File

A stream over an open file, from `fs.open`. It is also the type of
`os.stdin`, `os.stdout` and `os.stderr`, of the pipes of a command started
with `exec.start`, and of a `.gz` file opened with
[`gzip.open`](./module-gzip.md#streams). It shows as `<File notes.txt>`.

| Member | Kind | Value |
| --- | --- | --- |
| `path` | property | The Path it was opened with; `nil` for standard streams and pipes |
| `closed` | property | Bool |
| `read(size=nil)` | method | Bytes: up to `size` bytes, or everything left when `nil`. Empty Bytes at end of file |
| `read_line()` | method | The next line as a String without its terminator, or `nil` at end of file |
| `write(data)` | method | Writes a String or Bytes and returns the number of bytes written |
| `close()` | method | Releases the file; calling it again does nothing |
| `for line in file` | iteration | The remaining lines, as with `read_line()` |

Calling a method on a closed file, `read` on a file open for writing or
`write` on one open for reading raises ValueError. A line that is not valid
UTF-8 raises ValueError when it is reached.

The loop `for line in file` does not close the file: `read` and `read_line`
afterwards continue where the loop stopped. Whoever opened a file closes it,
normally with `defer file.close()`. Because a File has `read(size)` and
`write(data)`, it can be passed wherever a readable or a writable is accepted,
such as the `data` of `fs.write`.

### Reading lines

`fs.read_lines(path)` is the one-call way to go through a file line by line. Each
`for` over the value it returns opens the file, yields one line per step (the
`\n` or `\r\n` terminator removed, a last line without one included) and closes
the file when the loop ends by any route: at the end, on `break` or `return`,
or when an error is raised. It can be iterated again, which opens the file
again. A missing file raises NotExistError when the loop starts; a line that is
not valid UTF-8 raises ValueError `line 3 of "x" is not valid UTF-8`.

## FileInfo

An immutable snapshot from `fs.stat`. All members are properties.

| Property | Value |
| --- | --- |
| `size` | Int, bytes |
| `modified_at` | `time.Time` |
| `is_file` | Bool, a regular file |
| `is_dir` | Bool |
| `is_symlink` | Bool: the path itself is a symbolic link (the other properties describe its target) |
| `mode` | Int, permission bits (`info.mode == 0o644`) |

A FileInfo shows the kind of entry its properties describe:
`<FileInfo file size=12>` for a regular file, `<FileInfo dir>` for a
directory, `<FileInfo other>` for anything else (a device, a named pipe, a
socket). A symbolic link shows its target's kind, since the properties
describe the target; a broken link, which `stat` describes itself, shows as
`<FileInfo symlink>`.

`info.is_dir` reads a snapshot already taken, while `fs.is_dir(path)` asks the
disk again.

## Left out

Symlink creation, ownership, file locking, `seek`/`tell`, and changing the
working directory (it is process-global and unsafe with goblins; pass `cwd=` to
`exec` instead) are not part of the module. `walk` never yields directories.
