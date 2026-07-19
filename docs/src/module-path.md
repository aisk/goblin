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
