# Strings and collections

## Lists

Create a list with square brackets. Its elements may have different types.
Indices start at zero and can be read or assigned.

```goblin
var tasks = ["write", "test"]
print(tasks[0])
tasks[1] = "review"
tasks.push("ship")
print(tasks.pop()) # ship; it is removed from the list
```

Negative indexes count from the end. An index outside the list raises
`IndexError`.

```goblin
var colors = ["red", "green", "blue"]
print(colors[-1]) # blue
```

Common list methods include `size()`, `first()`, `last()`, `contains()`,
`join()`, `reverse()`, and `copy()`.

```goblin
var numbers = [1, 2, 3]
print(numbers.size())
print(numbers.join(","))
```

`push()` changes the existing list and returns it. `copy()` is useful when you
need a separate list before mutating it. List concatenation creates a new list:

```goblin
var first = [1, 2]
var combined = first + [3, 4]
print(combined) # [1, 2, 3, 4]
```

## Dictionaries

Create a dictionary with `{key: value}` and read or assign through a key. Keys
and values are runtime values.

```goblin
var user = {"name": "Ada", "age": 36}
print(user["name"])
user["active"] = true
```

Looking up a missing key with `dict[key]` raises `KeyError`. Use `get()` when a
missing value is expected, and provide a `default` when appropriate.

Dictionaries provide methods such as `size()`, `contains()`, `get()`, `keys()`,
`values()`, `items()`, and `update()`.

```goblin
var port = user.get("port", default=8080)
if user.contains("name") {
    print(user["name"])
}
```

`set_default(key, default)` returns the existing value or stores and returns
the default. `pop(key)` removes a key and returns its value. `items()` returns
a list of two-element `[key, value]` lists; dictionary iteration itself yields
only keys.

```goblin
var settings = {}
settings.set_default("theme", "dark")
for pair in settings.items() {
    print(pair[0], pair[1])
}
```

`List()` and `Dict()` construct the corresponding collections. See [Control
flow](./control-flow.md) for iteration. Collection elements may be mixed types,
but dictionary key equality uses the key's runtime value. Dictionary iteration
order is unspecified.
