# goblin

![logo](https://repository-images.githubusercontent.com/650676396/467e8cc2-8df2-445a-a477-b7bb399394a2)

Toy language built for fun.

## Installation

```sh
$ go install github.com/aisk/goblin@latest
```

## Hello world

```sh
$ cat hello.goblin
print("Hello, world!")

$ goblin run hello.goblin
Hello, world!
```

To compile a source file to a native executable:

```sh
$ goblin build-exe hello.goblin
$ ./hello
Hello, world!
```

## Learn Goblin in 5 Minutes

Goblin is a dynamically-typed language. The CLI can interpret a source file
directly with `goblin run`, or transpile and compile it with `goblin build-exe`.

```goblin
# Comments start with #

# Variables: int, float, string, bool, nil
var name = "Goblin"
var age = 1
var pi = 3.14
var cool = true
var nothing = nil

# Reassignment
age = 2

# Arithmetic (supports mixed int/float)
print(1 + 2 * 3)          # 7
print("ha" * 3)           # hahaha
print("hello" + " world") # hello world

# Comparisons and logic use truthiness and return booleans
print(1 < 2 && !false)    # true
print(0 || "fallback")    # true

# Control flow
if age > 1 {
    print(name, "is growing!")
} else if age == 1 {
    print("just born")
} else {
    print("not yet")
}

var i = 0
while i < 3 {
    i = i + 1
}

for x in [1, 2, 3] {
    print(x)
}

for i in range(0, 5) {
    print(i)
}

# Strings
var s = "hello"
print(s.size())   # 5
print(s.upper())  # HELLO
print(s.contains("ell"))

# Lists
var list = [1, 2, 3]
print(list[0])    # 1
list.push(4)
print(list.pop()) # 4
print(list.size())  # 3

# Dictionaries
var d = {"name": "Alice", "age": 30}
print(d["name"])  # Alice
d["city"] = "Paris"
print(d.size())   # 3

# Functions are first-class
func add(a, b) {
    return a + b
}
print(add(1, 2))  # 3

func apply(f, a, b) {
    return f(a, b)
}
print(apply(add, 3, 4))  # 7

# Function calls support positional, keyword, *args, and **kwargs
print(add(5, 6))       # 11
print(add(a=5, b=6))   # 11

func collect(prefix, *args, **kwargs) {
    print(prefix, args.size(), kwargs.size())
}
collect("n=", 1, 2, 3)
collect("n=", *range(0, 2))
collect(prefix="n=", **{"flag": true})

# Custom Types
type User(name, age=18) {
    func hello(self) {
        print(self.name)
    }
}

var alice = User("alice")
print(alice.name)   # "alice"
print(alice.age)    # 18
alice.hello()

var bob = User(name="bob", age=20)
print(bob.age)      # 20

# Error handling: raise an Error, recover it with try/catch
func checked_div(a, b) {
    if b == 0 {
        raise ZeroDivisionError.wrap("checked_div")
    }
    return a / b
}
try {
    checked_div(1, 0)
} catch e {
    print(e.message)               # checked_div: ZeroDivisionError
    print(e.is(ZeroDivisionError)) # true
}

# Errors are values built with Error(); wrap adds context, unwrap/is inspect the chain
var not_found = Error("not found")
var err = not_found.wrap("loading config")
print(err.message)          # loading config: not found
print(err.unwrap().message) # not found
print(err.is(not_found))    # true

# Predefined kinds are hierarchical: IndexError is a LookupError,
# ZeroDivisionError is an ArithmeticError, ParseError is a ValueError,
# and OS/network failures are IOError subclasses.
try {
    var x = [1, 2, 3][9]
} catch e {
    print(e.is(IndexError))  # true
    print(e.is(LookupError)) # true
}

# Built-ins include print, range, max, min, spawn, Error and typed constructors
print(max(1, 2, 3))  # 3
print(min(1, 2.5))   # 1
print(Int("42"))     # 42
print(Str(nil))      # none

# Concurrency uses Chan plus spawn()
var ch = Chan(0)
spawn(func() {
    ch.send("done")
})
print(ch.recv())     # done
ch.close()

# Standard modules must be imported before use
import "os"
import "json"

os.getenv("HOME")
os.getpid()
print(json.unmarshal("42"))

# Export
export name
export add
```

More examples are in the [`examples/`](examples/) directory. They are executable
tests, so they are the best source for exact output. For local module imports,
see [`examples/module_import.goblin`](examples/module_import.goblin).

## Grammar

Take a look at [goblin.bnf](https://github.com/aisk/goblin/tree/master/goblin.bnf).

## About the Project

Goblin is &copy; 2023-2026 by [AN Long](https://github.com/aisk).

### License

Goblin is distributed by a [MIT license](https://github.com/aisk/goblin/tree/master/LICENSE).
