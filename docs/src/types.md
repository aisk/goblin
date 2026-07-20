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

## Protocol methods

Goblin lets custom types participate in operations and protocols through
conventionally named methods such as `__add`, `__cmp`, `__bool`, `__str`,
`__iter`, and `__getitem`. These names have leading double underscores only;
there are no trailing underscores. Most programs can start with ordinary fields
and methods.

The most useful protocol methods are shown below. Their parameter shapes are
fixed: use `self` alone for conversion and iteration methods, `self, other` for
binary operators and comparison, and `self, index, value` for `__setitem`.

| Method | Enables |
| --- | --- |
| `__add`, `__sub`, `__mul`, `__div` | Arithmetic operators |
| `__and`, `__or`, `__not` | Logical `&&`, `||`, and `!` operators |
| `__cmp` | `==`, `!=`, `<`, `<=`, `>`, `>=`; return `-1`, `0`, or `1`. Consulted from either side of `==` |
| `__str` | Printing and `Str(value)` |
| `__bool` | Conditions and `Bool(value)` |
| `__iter` | `for value in instance` |
| `__getitem`, `__setitem` | `instance[index]` read and assignment |

If a protocol method is absent, the corresponding operation raises TypeError.
Equality is the exception: without `__cmp`, `==` and `!=` fall back to
identity, so an instance is equal only to itself and never raises. Ordering
comparisons (`<`, `<=`, `>`, `>=`) still require `__cmp`.

For example, this type supports `+` and printing:

```goblin
type Vector(x, y) {
    func __add(self, other) {
        return Vector(self.x + other.x, self.y + other.y)
    }

    func __str(self) {
        return "Vector(" + Str(self.x) + ", " + Str(self.y) + ")"
    }
}

print(Vector(1, 2) + Vector(3, 4))
```
