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

Use a bare `return` to exit early. It returns `nil`.

```goblin
func describe(value) {
    if value == nil {
        return
    }
    print(value)
}
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

The captured value belongs to the closure's surrounding scope. Each call to
`multiplier` above creates a separate function value, so `multiplier(3)` would
produce a different closure.

## Parameters

Calls may use positional arguments or parameter names:

```goblin
print(add(a=2, b=3))
```

Positional arguments must precede named arguments. A parameter can receive a
value only once; passing the same parameter positionally and by name is an
error.

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

The `*rest` parameter must be last, unless a final `**options` follows it.
`**options` must always be last. Expand a list or dictionary at a call site
with `f(*items)` or `f(**options)`:

```goblin
func add3(a, b, c) {
    return a + b + c
}

var values = [1, 2, 3]
print(add3(*values))
```

## Recursion and callbacks

A named function can call itself. Functions also work naturally as callbacks
because they are ordinary values.

```goblin
func factorial(n) {
    if n <= 1 {
        return 1
    }
    return n * factorial(n - 1)
}

func map_values(values, transform) {
    var result = []
    for value in values {
        result.push(transform(value))
    }
    return result
}

print(factorial(5))
print(map_values([1, 2, 3], func(x) { return x * x }))
```
