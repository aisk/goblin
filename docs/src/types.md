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
        return "(" + Str(self.x) + ", " + Str(self.y) + ")"
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

## Operators and conversions

A type takes part in operators, comparison, printing, conditions, loops and
indexing by implementing the built-in traits in `impl` blocks inside its body.
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
ordering, iteration and indexing. [Traits](./traits.md) covers the built-in
traits and how to declare traits of your own.
