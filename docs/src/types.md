# Types and methods

`type` defines a custom type with fields and methods. The parentheses list
fields supplied at construction; a field may have a default value. Every method
must declare `self` as its first parameter. Type definitions belong at module
scope, not inside a function or a control-flow block.

```goblin
type Point(x, y=0) {
    func move(self, dx, dy) {
        self.x = self.x + dx
        self.y = self.y + dy
    }

    func text(self) {
        return "(" + String(self.x) + ", " + String(self.y) + ")"
    }
}

var p = Point(1)
p.move(2, 3)
print(p.text())
```

Instance fields can be read and updated directly. Construction accepts both
positional and named arguments:

```goblin
var origin = Point(x=0, y=0)
origin.x = 10
```

Required fields must come before fields with defaults. Calling a type requires
all required fields exactly once; named arguments make construction clearer
when a type has several fields.

## Methods and state

Methods are ordinary functions attached to a type. They can read or replace
fields through `self`, and methods may call other methods on the same instance.

```goblin
type Counter(value=0) {
    func increment(self) {
        self.value = self.value + 1
        return self.value
    }
}

var counter = Counter()
print(counter.increment()) # 1
print(counter.increment()) # 2
```

An instance has exactly the fields and methods its type declares; it cannot
grow new ones. Where the type of a value is certain, a misspelled name is
reported before the program runs instead of raising AttributeError when the
line is reached:

```goblin
type Counter(value=0) {
    func reset(self) {
        self.valeu = 0      # error: Counter has no attribute 'valeu'
    }
}

var counter = Counter()
print(counter.total)        # error: Counter has no attribute 'total'
counter.reset(1)            # error: Counter.reset() takes 0 positional arguments, got 1
```

The type is certain for `self` inside the type's own methods and impl blocks,
for a construction such as `Counter().value`, and for a variable declared
with a construction and never assigned again. A value that arrives any other
way, such as a parameter or a list element, is checked when the access runs.

## Operators and conversions

A type takes part in operators, comparison, printing, conditions, loops and
indexing by implementing protocols in `impl` blocks inside its body.
An empty impl of `Eq`, `Ord`, `Hashable` or `Show` derives the behavior from
the fields:

```goblin
type Vector(x, y) {
    impl Eq {}
    impl Show {}
    impl Add {
        func add(self, other) {
            return Vector(self.x + other.x, self.y + other.y)
        }
    }
}

print(Vector(1, 2) + Vector(3, 4))      # Vector(x=4, y=6)
print(Vector(1, 2) == Vector(1, 2))     # true
```

Without any impl, an instance is equal only to itself, prints as
`<Vector@0x...>`, is always truthy, and raises TypeError for arithmetic,
ordering, iteration and indexing. [Protocols](./protocols.md) lists the
protocols and what each one enables.

## Checking a value's type

A type name such as `Point` is the constructor function for that type; it is
not a separate type object, and there is no `isinstance` built-in.
`type_of(value)` returns the function that builds values of the value's type,
so comparing it answers "is this a Point":

```goblin
type Point(x, y) {}

func describe(value) {
    if value == nil {
        return "nothing"
    }
    if type_of(value) == Point {
        return "a point"
    }
    if type_of(value) == Int || type_of(value) == Float {
        return "a number"
    }
    return "something else"
}

print(describe(Point(1, 2)))  # a point
print(describe(2.5))          # a number
print(describe(nil))          # nothing
```

Built-in values answer with the conversion functions: `Int`, `Float`, `String`,
`Bool`, `Bytes`, `List`, `Dict`, `Function`, `Error` and `Chan`. A value whose
type has no constructor (`nil`, a module, a Goblin handle) answers a
stand-in that still compares equal across values of that type and has its
`name`, such as `type_of(nil).name == "Nil"`. There is no question for
behaviour, such as whether a value implements a protocol: use the operation
itself, or compare `type_of(value)` with the type you expect.
