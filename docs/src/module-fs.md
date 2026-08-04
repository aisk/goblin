# fs

The fs module reads, writes, and inspects files and directories. Most functions
accept a string or Path as their path argument.

~~~goblin
import "fs"

fs.write("notes.txt", "remember this")
print(fs.read("notes.txt"))
print(fs.exists("notes.txt"))
fs.append("notes.txt", "\nnext line")
fs.remove("notes.txt")
~~~

Use read() and write() for whole-file work. write() replaces a file, while
append() adds text and returns the number of bytes written.

| Function | Purpose |
| --- | --- |
| open(path) / create(path) | Open an existing file or create one |
| read(path) / write(path, text) / append(path, text) | Whole-file text I/O |
| exists(path) | Check whether a path exists |
| stat(path) | Return name, size, and directory information |
| read_dir(path) | Return file-information entries |
| mkdir(path) / remove(path) | Create a directory or remove a path |

Files returned by open() or create() should be closed after use. Filesystem
operations can raise IOError.

## File objects

open(path) returns a read-oriented file object and create(path) returns a file
that can be written. File objects expose name, closed, read(), write(content),
stat(), and close(). read() consumes all remaining data and returns text;
write() accepts str or Bytes and returns the number of bytes written. Unlike an
HTTP response body, `File.read()` does not currently accept a size argument and
is not a streaming Reader-protocol implementation.

Because a File has a `write(data)` method, it already satisfies the writer
stream shape: it can be passed wherever the standard library accepts a writer
object, such as `exec.Command(stdout=...)` or the `dest=` keyword of
`csv.write_all`, `tar.write_all`, and `gzip.compress`.

~~~goblin
var file = fs.create("log.txt")
file.write("started\n")
print(file.name)
file.close()

var reader = fs.open("log.txt")
print(reader.read())
print(reader.stat().size)
reader.close()
fs.remove("log.txt")
~~~

Always close a file once its work is finished, including after a try/catch
block. For one small text file, fs.read() and fs.write() are simpler and avoid
managing a file object.

## Inspecting directories

stat(path) and read_dir(path) return FileInfo values. Their common fields are
name, size, is_dir, mode, and mod_time.

~~~goblin
var entries = fs.read_dir(".")
for entry in entries {
    if entry.is_dir {
        print("directory:", entry.name)
    } else {
        print("file:", entry.name, entry.size)
    }
}
~~~

mkdir() creates one directory only; it fails when the parent is missing or the
path already exists. remove() removes one file or empty directory.
