# Collections

Lists and dictionaries are mutable collections. Their elements can have mixed
runtime types. Empty collections are false in conditions.

## Lists

Create a list with square brackets. Indexes begin at zero, and a negative
index counts from the end: `-1` is the last element and `-size` the first.

~~~goblin
var tasks = ["write", "test"]
print(tasks[0])
tasks[-1] = "review"
tasks.push("ship")
print(tasks.pop())
~~~

An index outside `-size <= i < size` raises IndexError. The + operator makes a new
concatenated list; push(), reverse(), sort(), and clear() change the current
list.

~~~goblin
var first = [1, 2]
var combined = first + [3, 4]
print(combined) # [1, 2, 3, 4]
~~~

### Slicing

`list[low:high]` makes a new list of the elements from index `low` up to, but
excluding, `high`. Either bound may be left out: `low` defaults to the start
and `high` to the end.

~~~goblin
var numbers = [10, 20, 30, 40, 50]
print(numbers[1:3]) # [20, 30]
print(numbers[:2])  # [10, 20]
print(numbers[3:])  # [40, 50]
print(numbers[:])   # a copy of the whole list
~~~

A slice is a copy, so changing it never changes the list it came from. A bound
names a position between elements, so it may also equal the size; like an
index, a negative bound counts from the end, so `numbers[:-1]` drops the last
element and `numbers[-2:]` keeps the last two. A bound outside
`-size <= bound <= size`, or a `low` that ends up past `high`, raises
IndexError; bounds are never clamped. Strings and Bytes are sliced the same
way. A slice cannot be assigned to.

The same two kinds of position appear in methods: pop() takes an element
index, while insert() and the `start` of index() take a bound, so
`list.insert(list.size, value)` appends.

### List method guide

| Method | Purpose |
| --- | --- |
| size | Number of elements (property) |
| first / last | The first or last element, or nil for an empty list (properties) |
| push(value, ...) | Append one or more values |
| pop(index=-1) | Remove and return an element; defaults to the last |
| insert(index, value) | Insert before a position; `size` appends |
| remove(value) | Remove the first matching value, if there is one |
| clear() | Remove every element |
| reverse() | Reverse the order in place |
| sort(key=nil, descending=false) | Sort in place, stably |
| copy() | A new list with the same elements |
| contains(value) / count(value) | Test for or count a value |
| index(value, start=0) | Position of the first matching value, or nil |
| find(fn) | The first element `fn` is true for, or nil |
| join(sep="") | Join the elements' text, or Bytes with a Bytes separator |
| map(fn) / filter(fn) / reduce(fn, initial) | Transform, select or combine elements |
| any(fn=nil) / all(fn=nil) | Whether some or every element (or `fn` of it) is true |

Every method that changes the list (push, insert, remove, clear, reverse and
sort) returns the list itself, so calls chain:
`names.push("Cy").sort().reverse()`. pop() is the exception: it returns the
element it removed, and raises IndexError for an empty list. pop() accepts a
negative index, so its default `-1` removes the last element. first and last
are nil for an empty list, and remove() of a value that is not there does
nothing.

index() returns nil when a value is absent. Position `0` is false in a
condition, so test the result with `!= nil`, or use contains() when the
position does not matter.

sort() is always stable: elements that compare equal (or whose keys do) keep
their order, with `descending=true` as well. `descending` must be a Bool.
Arguments that are functions (`fn`, `key`) are checked when the method is
called, so `xs.map(1)` raises TypeError before doing anything.

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
var ordered = original.copy().sort()
print(original) # [3, 1, 2]
print(ordered)  # [1, 2, 3]
~~~

Use contains() for membership tests. Use index() when the position matters;
check for a missing value before using its result as an index.

Lists compare lexicographically with `<`, `<=`, `>` and `>=`: the first
unequal pair of elements decides, and a list that is a prefix of another sorts
first. This makes a list a convenient sort key over several fields:

~~~goblin
var people = [["Bo", 30], ["Al", 30], ["Cy", 25]]
people.sort(key=func(p) { return [p[1], p[0]] })
print(people) # [["Cy", 25], ["Al", 30], ["Bo", 30]]
~~~

### Transforming lists with callbacks

Lists also provide callback-based helpers. map() returns transformed values,
filter() keeps values whose callback is truthy, and reduce() combines values
from left to right. find() returns the first matching value or nil. To run a
callback for its side effect, write a `for` loop.

~~~goblin
var values = [1, 2, 3, 4]
var squares = values.map(func(value) { return value * value })
var large = values.filter(func(value) { return value > 2 })
var total = values.reduce(func(acc, value) { return acc + value }, 0)

print(squares) # [1, 4, 9, 16]
print(large)   # [3, 4]
print(total)   # 10
~~~

Callbacks receive one list element, except reduce(), whose callback receives
the accumulator followed by the element. Without an initial value, reduce()
uses the first list element and raises ValueError for an empty list. any() and
all() accept an optional callback; with none, they test each element's
truthiness. To add up the elements, use the global `sum(values)`; `max` and
`min` likewise take a list (see [Built-in functions](./built-in-functions.md)).

## Dictionaries

Create a dictionary with key: value pairs. Read and write through a key.

~~~goblin
var user = {"name": "Ada", "age": 36}
print(user["name"])
user["active"] = true
print(user.get("role", default="reader"))
~~~

Keys may be strings, Bytes, integers, floats, booleans, nil, or any other value that
implements `Hashable`. Key identity follows `==`: keys of different types stay
distinct (`d[1]` and `d["1"]` are separate entries), while equal numbers name
the same entry (`d[1]` and `d[1.0]` are one key). Values whose equality is
identity, such as functions, error kinds and channels, hash by
identity. Standard library values like `Time`, `Path` and `URL` hash by the
value they compare equal on. Using a list, dictionary,
or a user type without an `impl Hashable` as a key raises TypeError.

Looking up a missing key with dictionary[key] raises KeyError. Use get() when a
missing value is expected. Dictionary iteration yields keys; items() yields
two-element [key, value] lists, which a loop can destructure.

A dictionary remembers insertion order: iteration, keys(), values(), items(),
copy(), update() and printing all list the keys in the order they were first
written. Writing to a key that is already present keeps its place; removing a
key and writing it again moves it to the end. Equality ignores order, so
`{"a": 1, "b": 2} == {"b": 2, "a": 1}`.

~~~goblin
var settings = {}
settings.get_or_set("theme", "dark")
for key, value in settings.items() {
    print(key, value)
}
~~~

### Dictionary method guide

| Method | Purpose |
| --- | --- |
| size | Number of entries (property) |
| contains(key) | Test whether a key exists |
| get(key, default=nil) | Read a key without raising for a missing key |
| get_or_set(key, default) | Get an existing value, or store `default` and return it |
| keys() / values() / items() | Return lists of keys, values, or key/value pairs |
| pop(key, default) | Remove and return a value; KeyError without a default |
| remove(key) | Remove a key if it is present |
| update(other) | Copy entries from another dictionary |
| copy() / clear() | Duplicate or empty the dictionary |

remove(), update() and clear() return the dictionary itself, so they chain.

List() and Dict() construct the corresponding collections. See [Control
flow](./control-flow.md) for iteration.

### Common dictionary patterns

Use get() to read optional configuration, get_or_set() to initialize a value
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
