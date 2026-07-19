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

Common list methods include `size()`, `first()`, `last()`, `contains()`,
`join()`, `reverse()`, and `copy()`.

```goblin
var numbers = [1, 2, 3]
print(numbers.size())
print(numbers.join(","))
```

## Dictionaries

Create a dictionary with `{key: value}` and read or assign through a key. Keys
and values are runtime values.

```goblin
var user = {"name": "Ada", "age": 36}
print(user["name"])
user["active"] = true
```

Dictionaries provide methods such as `size()`, `contains()`, `get()`, `keys()`,
`values()`, `items()`, and `update()`.

```goblin
var port = user.get("port", default=8080)
if user.contains("name") {
    print(user["name"])
}
```

`List()` and `Dict()` construct the corresponding collections. See [Control
flow](./control-flow.md) for iteration.
