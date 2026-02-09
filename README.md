# goblin

![logo](https://repository-images.githubusercontent.com/650676396/467e8cc2-8df2-445a-a477-b7bb399394a2)

Toy language built for fun.

## Installation

```sh
$ go install github.com/aisk/goblin/command/goblin@latest
```

## Hello world

```sh
$ cat hello.goblin
print("Hello, world!")

$ goblin hello.goblin > hello.go

$ go build hello.go

$ ./hello
"Hello, world!"
```

## Learn Goblin in 5 Minutes

Goblin is a dynamically-typed language that transpiles to Go.

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
print("ha" * 3)           # "hahaha"
print("hello" + " world") # "hello world"

# Comparisons and logic
print(1 < 2 && !false)   # true
print(0 || "fallback")   # "fallback" (truthy/falsey)

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
print(s.size)     # 5
print(s.upper())  # "HELLO"

# Lists
var list = [1, 2, 3]
print(list[0])    # 1
list.push(4)
list.pop()
print(list.size)  # 3

# Dictionaries
var d = {"name": "Alice", "age": 30}
print(d["name"])   # "Alice"
print(d.keys())    # ["name", "age"]

# Functions are first-class
func add(a, b) {
    return a + b
}
print(add(1, 2))  # 3

func apply(f, a, b) {
    return f(a, b)
}
print(apply(add, 3, 4))  # 7

# Built-in functions: print, range, max, min
print(max(1, 2, 3))  # 3
print(min(1, 2.5))   # 1

# Modules
os.getenv("HOME")
os.getpid()

# Export
export name
export add
```

More examples in the [`examples/`](examples/) directory.

## Grammar

Take a look at [goblin.bnf](https://github.com/aisk/goblin/tree/master/goblin.bnf).

## About the Project

Goblin is &copy; 2023-2026 by [AN Long](https://github.com/aisk).

### License

Goblib is distributed by a [MIT license](https://github.com/aisk/goblin/tree/master/LICENSE).
