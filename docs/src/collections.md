# Collections

Lists and dictionaries are mutable collections. Their elements can have mixed
runtime types. Empty collections are false in conditions.

## Lists

Create a list with square brackets. Indexes begin at zero, and negative indexes
count from the end.

~~~goblin
var tasks = ["write", "test"]
print(tasks[0])
print(tasks[-1])
tasks[1] = "review"
tasks.push("ship")
print(tasks.pop())
~~~

An index outside the list raises IndexError. The + operator makes a new
concatenated list; push(), reverse(), sort(), and clear() change the current
list.

~~~goblin
var first = [1, 2]
var combined = first + [3, 4]
print(combined) # [1, 2, 3, 4]
~~~

### List method guide

| Method | Purpose |
| --- | --- |
| size() | Number of elements |
| push(value, ...) | Append one or more values |
| pop(index=-1) | Remove and return an element; defaults to the last |
| first() / last() | Read the first or last element without removing it |
| insert(index, value) | Insert before an index |
| remove(value) | Remove the first matching value |
| contains(value) / count(value) | Test for or count a value |
| index(value, start=0) | Find a value from an offset |
| join(separator) | Convert elements to text with a separator |
| reverse() / sort() | Reorder the list in place |
| copy() / clear() | Duplicate or empty the list |

pop(), first(), last(), and index() can raise IndexError when the requested
element is unavailable. remove() raises ValueError if the value is absent.

### Common list patterns

Use push() while building a result, pop() when processing a work stack, and
copy() before an operation that should not change the original list.

~~~goblin
var names = ["Ada", "Linus", "Grace"]
var greetings = []
for name in names {
    greetings.push("Hello, " + name)
}
print(greetings.join(" | "))

var original = [3, 1, 2]
var ordered = original.copy()
ordered.sort()
print(original) # [3, 1, 2]
print(ordered)  # [1, 2, 3]
~~~

Use contains() for membership tests. Use index() when the position matters;
check for a missing value before using its result as an index.

## Dictionaries

Create a dictionary with key: value pairs. Read and write through a key.

~~~goblin
var user = {"name": "Ada", "age": 36}
print(user["name"])
user["active"] = true
print(user.get("role", default="reader"))
~~~

Looking up a missing key with dictionary[key] raises KeyError. Use get() when a
missing value is expected. Dictionary iteration yields keys; items() yields
two-element [key, value] lists. Dictionary iteration order is unspecified.

~~~goblin
var settings = {}
settings.set_default("theme", "dark")
for pair in settings.items() {
    print(pair[0], pair[1])
}
~~~

### Dictionary method guide

| Method | Purpose |
| --- | --- |
| size() | Number of entries |
| contains(key) | Test whether a key exists |
| get(key, default=nil) | Read a key without raising for a missing key |
| set_default(key, default=nil) | Get an existing value or insert a default |
| keys() / values() / items() | Return lists of keys, values, or key/value pairs |
| pop(key) | Remove and return a value |
| update(other) | Copy entries from another dictionary |
| copy() / clear() | Duplicate or empty the dictionary |

List() and Dict() construct the corresponding collections. See [Control
flow](./control-flow.md) for iteration.

### Common dictionary patterns

Use get() to read optional configuration, set_default() to initialize a value
once, and update() to merge a set of overrides into a base dictionary.

~~~goblin
var defaults = {"host": "127.0.0.1", "port": 8080}
var overrides = {"port": 3000}
defaults.update(overrides)
print(defaults["port"]) # 3000

var counts = {}
for word in ["go", "goblin", "go"] {
    var current = counts.get(word, default=0)
    counts[word] = current + 1
}
print(counts["go"]) # 2
~~~

Use keys(), values(), or items() only when a list snapshot is useful. For a
simple dictionary traversal, iterate the dictionary itself and look up each
value by its key.
