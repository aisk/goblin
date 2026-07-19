# Types and methods

`type` defines a custom type with fields and methods. The parentheses list
fields supplied at construction; a field may have a default value. Every method
must declare `self` as its first parameter.

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

Goblin lets custom types participate in operations and protocols through
conventionally named methods such as `__add`, `__cmp`, `__bool`, `__str`,
`__iter`, and `__getitem`. These names have leading double underscores only;
there are no trailing underscores. Most programs can start with ordinary fields
and methods.
