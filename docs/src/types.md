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
| `__radd`, `__rsub`, `__rmul`, `__rdiv` | The same operators with the instance on the right |
| `__not` | Logical `!` operator; without it `!` negates truthiness |
| `__cmp` | `==`, `!=`, `<`, `<=`, `>`, `>=`; return `-1`, `0`, or `1`. Consulted from either side of a comparison |
| `__str` | Printing and `Str(value)` |
| `__bool` | Conditions and `Bool(value)` |
| `__iter` | `for value in instance` |
| `__getitem`, `__setitem` | `instance[index]` read and assignment |

If a protocol method is absent, the corresponding operation raises TypeError.
Equality is the exception: without `__cmp`, `==` and `!=` fall back to
identity, so an instance is equal only to itself and never raises. Ordering
comparisons (`<`, `<=`, `>`, `>=`) still require `__cmp`.

With `__cmp`, a type mismatch inside it still reads as "unequal" — `money ==
nil` stays false for a `__cmp` written only for numbers — but any other failure
is reported rather than swallowed, so `==` raises whatever the method raised.

A comparison consults both operands, so the instance does not have to be on the
left. When the left operand has no ordering for the right one — as a built-in
number never has for a custom type — the right operand is asked to compare
itself against the left and the result is flipped:

```goblin
type Money(amount) {
    func __cmp(self, other) {
        return self.amount - other
    }
}

var m = Money(5)
print(m < 10) # true
print(10 > m) # true, answered by Money.__cmp
```

Arithmetic cannot flip its operands the same way — `a - b` is not `b - a` — so
it uses a second set of methods instead. When the left operand does not know
the right one, the right operand's `__radd`, `__rsub`, `__rmul` or `__rdiv` is
called with the left operand as its argument:

```goblin
type Scaled(factor) {
    func __mul(self, other) {
        return Scaled(self.factor * other)
    }
    func __rmul(self, other) {
        return Scaled(other * self.factor)
    }
}

var s = Scaled(3)
print(s * 2) # Scaled(6)
print(2 * s) # Scaled(6), answered by __rmul
```

A reflected method is only reached after the left operand reports that the
types do not fit. Anything else it raises — a division by zero, a failure
inside its own body — propagates instead, and an operator whose reflected
method is not defined keeps reporting the error of its left operand.

The built-in sequences follow the same rule, so repetition reads in either
order: `3 * "ab"` and `2 * [1, 2]` work like `"ab" * 3` and `[1, 2] * 2`.

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
