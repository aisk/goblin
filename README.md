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

## Learn Goblin in Y Minutes

Goblin is a simple dynamically-typed language that transpiles to Go. This guide covers the syntax and features of the language.

### Comments

```goblin
# This is a comment
print("before")
# Another comment
print("after")
```

### Variables

Variables are declared with `var` and can be reassigned using `=`:

```goblin
var a = 123
print(a)  # 123
a = 456
print(a)  # 456

# Different types
var s = "hello"
print(s)  # "hello"

var b = true
print(b)  # true

var f = 3.14
print(f)  # 3.14

var n = nil
print(n)  # nil
```

### Arithmetic

Goblin supports integer and float arithmetic:

```goblin
# Integer arithmetic
print(1 + 2)   # 3
print(10 - 3)  # 7
print(4 * 5)   # 20
print(10 / 2)  # 5

# Operator precedence
print(1 + 2 * 3)    # 7
print((1 + 2) * 3)  # 9

# Float arithmetic
print(3.14)        # 3.14
print(2.5 + 1.5)   # 4.0
print(5.0 * 2.0)   # 10.0
print(10.0 / 2.0)  # 5.0

# Mixed int/float
print(3.5 + 2)     # 5.5
print(10 / 2.5)    # 4.0
```

### Strings

Strings support concatenation, methods, iteration, and multiplication:

```goblin
# String concatenation
print("hello" + " " + "world")  # "hello world"

# String methods
var s = "hello"
print(s.size)     # 5
print(s.upper())  # "HELLO"
print(s.lower())  # "hello"

# String iteration
for ch in "abc" {
    print(ch)  # prints "a", "b", "c"
}

# String multiplication
print("ha" * 3)  # "hahaha"
```

### Boolean Logic

Goblin supports `&&`, `||`, and `!` operators with truthiness semantics:

```goblin
# Basic AND/OR
print(true && true)   # true
print(true && false)  # false
print(false || true)  # true
print(false || false) # false

# NOT operator
print(!true)   # false
print(!false)  # true
print(!!true)  # true

# Truthiness with integers
print(1 && 0)  # 0 (falsey)
print(1 || 0)  # 1 (truthy)
print(0 || 1)  # 1

# Truthiness with strings
print("hello" && "")  # "" (falsey)
print("" || "hello")  # "hello" (truthy)

# Compound expressions
print((1 && 0) || true)  # true
print(!false && true)    # true
```

### Comparisons

```goblin
# Integer comparisons
print(1 == 1)   # true
print(1 != 2)   # true
print(1 < 2)    # true
print(2 > 1)    # true
print(1 <= 1)   # true
print(1 >= 1)   # true

# Float comparisons
print(1.5 < 2.5)    # true
print(1.0 == 1.0)   # true

# String comparisons
print("abc" == "abc")  # true
print("abc" < "def")   # true

# Bool comparisons
print(true == true)    # true
print(true != false)   # true

# Nil comparison
print(nil == nil)  # true
```

### Control Flow

#### If/Else

```goblin
# if
if true {
    print("if works")
}

# if/else
if false {
    print("wrong")
} else {
    print("else works")
}

# if/else if/else
if false {
    print("wrong")
} else if true {
    print("else if works")
} else {
    print("wrong")
}
```

#### While Loop

```goblin
var count = 0
while count < 5 {
    print(count)
    count = count + 1
}

# while + break
while true {
    print(count)
    count = count + 1
    if count == 3 {
        break
    }
}
```

#### For Loop

```goblin
# for-in list
for x in [1, 2, 3] {
    print(x)
}

# for-in string
for ch in "hi" {
    print(ch)
}

# Nested for loop
var list = [[1, 2], [3, 4]]
for inner in list {
    for x in inner {
        print(x)
    }
}

# for-in with range()
for i in range(0, 3) {
    print(i)  # 0, 1, 2
}
```

### Lists

```goblin
# List creation and indexing
var list = [1, 2, 3]
print(list)     # [1, 2, 3]
print(list[0])  # 1
print(list[1])  # 2
print(list[2])  # 3

# Nested list indexing
var nested = [[1, 2], [3, 4]]
print(nested[0][1])  # 2
print(nested[1][0])  # 3

# List methods
print(list.size)  # 3
list.push(4)
print(list.size)  # 4
print(list.pop()) # 4
print(list)       # [1, 2, 3]
```

### Dictionaries

```goblin
# Dict creation
var empty = {}
print(empty)  # {}

var d = {"name": "Alice", "age": 30}
print(d)  # {"name": "Alice", "age": 30}

# Dict indexing
print(d["name"])  # "Alice"
print(d["age"])   # 30

# Dict methods
print(d.size)      # 2
print(d.keys())    # ["name", "age"]
print(d.values())  # ["Alice", 30]

# Dict iteration
for key in d {
    print(key)  # "name", "age"
}
```

### Functions

Functions are first-class citizens:

```goblin
# Basic function
func hello() {
    print("hello!")
}
hello()

# Function with params and return
func add(a, b) {
    return a + b
}
print(add(1, 2))  # 3

# Function with string param
func greet(name) {
    print("hello", name)
}
greet("world")  # prints "hello world"

# First-class function
func apply(f, a, b) {
    return f(a, b)
}
print(apply(add, 3, 4))  # 7
```

### Built-in Functions

```goblin
# print - outputs values
print("hello", 42, 3.14)

# range - generates a list of integers
var numbers = range(0, 5)
print(numbers)  # [0, 1, 2, 3, 4]

# max - returns the maximum value
print(max(1, 2, 3))     # 3
print(max(1.5, 2.5))    # 2.5
print(max(1, 2.5))      # 2.5 (mixed types)

# min - returns the minimum value
print(min(1, 2, 3))     # 1
print(min(1.5, 2.5))    # 1.5
print(min(1, 2.5))      # 1 (mixed types)
```

### Modules

Goblin supports importing modules:

```goblin
# os module - system functions
os.getenv("HOME")   # get environment variable
os.getpid()         # get process ID
os.getppid()        # get parent process ID
os.getuid()         # get user ID
# os.exit(code)     # exit with code (terminates program)
```

### Export

You can export variables and functions from a module:

```goblin
var x = 42
func add(a, b) {
    return a + b
}
var internal = 100

export x
export add
# internal is not exported
```

## Grammar

Take a look at [goblin.bnf](https://github.com/aisk/goblin/tree/master/goblin.bnf).

## About the Project

Goblin is &copy; 2023-2026 by [AN Long](https://github.com/aisk).

### License

Vox is distributed by a [MIT license](https://github.com/aisk/goblin/tree/master/LICENSE).
