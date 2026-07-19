# path

The path module provides a Path value and directory factories. Use Path rather
than manually joining strings when code must work with filesystem paths.

~~~goblin
import "path"
import "fs"

var config = path.home().join("myapp", "config.json")
print(config)
print(fs.exists(config))
~~~

path.cwd() returns the current working directory and path.home() returns the
user home directory. path.Path(text) constructs a Path explicitly.

Path values expose operations such as join(), name, parent, suffix, exists(),
is_dir(), and read_text() or write_text() when working directly with a path.
Use fs when a program prefers functional whole-file operations; use Path when
several operations are derived from one base location.

## Building derived paths

join() makes a child path without manually inserting separators. name, stem,
suffix, parent, parts, and is_absolute describe a path without touching the
filesystem.

~~~goblin
var source = path.Path("reports/monthly.csv")
print(source.name)   # monthly.csv
print(source.stem)   # monthly
print(source.suffix) # .csv
print(source.parent)
~~~

with_name(name) and with_suffix(suffix) create adjusted paths. relative_to()
and as_posix() are useful when producing portable display strings.

## Filesystem operations on Path

Path can directly test exists(), is_file(), is_dir(), and is_symlink(). It can
read_text(), write_text(text), read_bytes(), write_bytes(bytes), mkdir(),
unlink(), rename(target), iterdir(), and glob(pattern).

~~~goblin
var output = path.cwd().join("output.txt")
output.write_text("generated")
if output.exists() {
    print(output.read_text())
}
output.unlink()
~~~

These operations can raise IOError. Use fs.read_dir() for simple directory
listing or Path.glob() when a pattern is the clearest expression.
