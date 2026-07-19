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
