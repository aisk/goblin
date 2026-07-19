# Functions

Define a function with `func`. Parameters do not have declared types, and
`return` sends a result back to the caller. A function without a value returns
`nil`.

```goblin
func add(a, b) {
    return a + b
}

print(add(2, 3))
```

Functions are values: assign them, pass them to another function, or return
them. An anonymous function omits the name.

```goblin
var square = func(x) { return x * x }

func apply(f, value) {
    return f(value)
}

print(apply(square, 5))
```

An anonymous function captures variables in its surrounding scope, so it can
form a closure:

```goblin
func multiplier(n) {
    return func(x) { return x * n }
}

var double = multiplier(2)
print(double(21))
```

## Parameters

Calls may use positional arguments or parameter names:

```goblin
print(add(a=2, b=3))
```

A `*` parameter collects extra positional arguments, while a `**` parameter
collects extra named arguments. They receive a list and a dictionary,
respectively.

```goblin
func show(first, *rest, **options) {
    print(first)
    print(rest)
    print(options)
}

show("a", "b", "c", color="green")
```

Expand a list or dictionary at a call site with `f(*items)` or `f(**options)`.
