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

A parameter can declare a default value with `=`, making it optional at the
call site:

```goblin
func greet(name, msg = "hi") {
    print(msg, name)
}

greet("bob")                # hi bob
greet("bob", msg = "yo")    # yo bob
```

A required parameter cannot follow one with a default. The default expression
is evaluated in the function's defining scope — it cannot reference other
parameters — and it is evaluated on each call that omits the argument, so a
mutable default like `[]` produces a fresh value every time.

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

A `**`-expanded value must be a dictionary with string keys, and each keyword
name may only be supplied once per call: naming an argument twice — whether by
two explicit keywords, two `**` dictionaries that share a key, or one of each —
raises a TypeError.

### Calls checked before the program runs

When a call names a function directly, its arguments are matched against the
parameter list before the program starts, by `goblin run` and `goblin
build-exe` alike:

```goblin
func area(width, height = 1) {
    return width * height
}

area()             # error: area() missing required positional argument: 'width'
area(1, 2, 3)      # error: area() takes 2 positional arguments, got 3
area(1, depth=2)   # error: area() got an unexpected keyword argument 'depth'
area(1, width=2)   # error: area() got multiple values for argument 'width'
```

This covers names bound by `func`, by `type` (the constructor), and by a
`var` initialized with a function literal, as long as the name is never
assigned anywhere in the module. A `*` or `**` argument supplies an unknown
number of values, so only mistakes that no expansion could repair are
reported. Every other call — through a parameter, a reassigned name, a list
element, a function from another module — is matched when it runs and raises
the same message as a TypeError. The REPL does no checking ahead of time.

## Deferred calls

`defer` schedules a call for the moment the enclosing function ends, however
it ends: a `return`, running off the end of the body, or an error. Put it
right after acquiring something that has to be released.

```goblin
import "fs"

func first_line(path) {
    var file = fs.open(path)
    defer file.close()
    return file.read().decode().split("\n")[0]
}
```

`defer` must be followed by a call. The function being called and its
arguments are evaluated when the `defer` statement runs; only the call itself
waits. Defer a function literal when the work should see the variables as
they are at the end instead:

```goblin
func demo() {
    var label = "before"
    defer print("argument:", label)                 # argument: before
    defer func() { print("closure:", label) }()     # closure: after
    label = "after"
}
```

Deferred calls run in reverse order, the last one deferred first, so things
are released in the opposite order to how they were acquired. In the example
above the closure line prints first.

A few rules follow from "when the function ends":

- A `defer` inside an `if`, a loop, or a `try` block still belongs to the
  function, not the block. One in a loop adds a call on every iteration, and
  all of them wait until the function returns.
- The result of a deferred call is discarded, and it cannot change the value
  the function has already decided to return.
- If a deferred call raises, that error replaces whatever the function was
  returning or raising. The remaining deferred calls still run.
- At the top level of a file, deferred calls run when the file's own
  statements finish, also when they stop with an unhandled error. In the REPL
  that is the end of each input. `os.exit()` ends the process without running
  them.

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

Recursion depth is limited by the stack a goroutine may use (256 MB), which
allows hundreds of thousands of nested calls. A recursion that runs past it,
usually one missing its base case, raises RecursionError instead of crashing
the process, and can be caught like any other error. Comparing or printing
values nested that deeply, such as two distinct lists that each contain
themselves, raises the same error. The traceback folds the repeated frames
into a single "previous frame repeated N more times" line.

```goblin
func forever(n) {
    return forever(n + 1)
}

try {
    forever(0)
} catch err {
    print(err.is(RecursionError)) # true
}
```
