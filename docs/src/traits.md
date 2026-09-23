# Traits

A trait is a named bundle of methods that a type can implement. Goblin's
operators and conversions are traits too: a type supports `+` by implementing
`Add`, `==` by implementing `Eq`, printing by implementing `Show`, and so on.

~~~goblin
trait Shape {
    func area(self)                     # required: no body
    func describe(self) {               # default: has a body
        return "area " + Str(Shape.area(self))
    }
}

type Rect(w, h) {
    impl Eq {}                          # empty impl: structural equality
    impl Show {}                        # empty impl: Rect(w=2, h=3)
    impl Shape {
        func area(self) {
            return self.w * self.h
        }
    }
}

var r = Rect(2, 3)
print(r)                                # Rect(w=2, h=3)
print(r == Rect(2, 3))                  # true
print(Shape.describe(r))                # area 6
print(r.traits())                       # [<trait Eq>, <trait Show>, <trait Shape>]
~~~

## Declaring a trait

`trait Name { ... }` declares a trait at module scope. Each member is a method
whose first parameter is `self`. A method without a body is required; a method
with a body is a default that implementations may override. Trait methods take
a fixed parameter list: no defaults, `*args` or `**kwargs`.

A trait may name other traits after a colon. A type implementing it must then
implement those as well:

~~~goblin
trait Named {
    func name(self)
}

trait Titled: Named, Show {
    func title(self)
}
~~~

A trait is a value bound to its name, like a type. `export Shape` makes it
importable, and another module implements it as `impl shapes.Shape { ... }`.
A trait must be declared before the types that implement it.

## Implementing a trait

An `impl` block sits inside a type body, next to ordinary methods and in any
order. A type has at most one impl per trait. The block must define every
required method with the declared parameter count, may override defaults, and
may not define anything else.

A trait with no required methods, a mixin, accepts an empty impl:

~~~goblin
trait Loud {
    func shout(self) {
        return Str(self).upper() + "!"
    }
}

type Word(text) {
    impl Show {
        func show(self) {
            return self.text
        }
    }
    impl Loud {}
}

print(Loud.shout(Word("hey")))         # HEY!
~~~

These rules are checked before the program runs. For instance, an impl
missing a required method reports `impl Shape for Rect is missing method
'area'`.

## Calling trait methods

Trait methods never become attributes of the instance: `r.area()` does not
exist, and an ordinary method or field may reuse a trait method's name. A trait
method is called through the trait, with the receiver as the first argument:

~~~goblin
print(Shape.area(r))
print(Ord.max(3, 7))                    # built-in traits work on built-in values
~~~

The call runs the receiver's implementation, or the trait's default when the
impl does not override it. A value without an impl raises
`TypeError: Integer does not implement Shape`, and a wrong argument count
raises TypeError too.

Default methods call their siblings the same way, as `Shape.area(self)`.
`value.traits()` returns the traits a user type implements, in declaration
order, so `r.traits().contains(Shape)` answers whether `r` is a Shape.

## Built-in traits

| Trait | Required | Defaults | Enables | Empty impl |
| --- | --- | --- | --- | --- |
| `Eq` | `eq(self, other)` | `ne` (derived) | `==`, `!=` | structural |
| `Ord` (needs `Eq`) | `compare(self, other)` | `lt le gt ge max min` (derived) | `<`, `<=`, `>`, `>=`, `sort`, `max`, `min` | structural |
| `Hashable` (needs `Eq`) | `hash(self)` | | dict keys | structural |
| `Show` | `show(self)` | | `print`, `Str`, string rendering | structural |
| `Truth` | `truth(self)` | | `if`, `while`, `&&`, `\|\|`, `!`, `Bool` | not allowed |
| `Add` `Sub` `Mul` `Div` `Mod` | `add` `sub` `mul` `div` `mod` `(self, other)` | `radd` `rsub` `rmul` `rdiv` `rmod` | `+ - * / %` | not allowed |
| `Neg` | `neg(self)` | | unary `-` | not allowed |
| `Iter` | `iter(self)` | | `for x in value` | not allowed |
| `Index` | `get(self, index)` | `set(self, index, value)` | `value[i]`, `value[i] = x` | not allowed |

Derived defaults are defined by the required method and cannot be overridden,
so `<`, `sort` and `max` can never disagree about an order. Defining one in an
impl is an error: `impl Ord for Point: method 'lt' derives from the required
methods and cannot be overridden`.

`compare` returns a negative Integer, zero, or a positive Integer. `compare` and
`hash` must return an Integer, `show` a String, and `eq`, `ne`, `truth` and the
ordering methods a Bool; anything else raises
`TypeError: Point.show must return String, got Integer`.

Without an impl, the defaults are: `==` is identity, printing shows
`<Point@0x...>`, a value is truthy, and ordering, arithmetic, iteration,
indexing and use as a dict key raise TypeError.

### Structural implementations

An empty impl of `Eq`, `Ord`, `Hashable` or `Show` gets the structural
implementation, which works on the fields in declaration order:

- `Eq`: equal when the other value has the same type and every field pair is
  equal.
- `Ord`: lexicographic, the first unequal field pair decides.
- `Hashable`: combines the type and the hash of every field. A field that is
  not hashable, such as a List, raises TypeError when the value is hashed.
- `Show`: `Point(x=1, y="a")`, with fields rendered the way collections
  render their elements.

A structural `Ord` or `Hashable` must sit on a structural `Eq`: a structural
hash cannot know which fields a custom `eq` ignores. The checker reports
`structural Hashable requires structural Eq on Point` otherwise.

### Eq and Ord

`Ord` depends on `Eq`, and like every dependency the type implements it
explicitly. A structural `Ord` pairs with `impl Eq {}`; a custom `compare`
usually pairs with an `eq` that asks it, as `Money` does below, so that `==`
agrees with the ordering.

Equality never raises over a type mismatch. `a == b` asks the left operand's
`eq`, then the right one's, and falls back to identity; a TypeError raised
inside an `eq` means "unequal", so `money == nil` stays false for an `eq`
written only for numbers. Any other error propagates. `!=` is always the
negation of `==`.

The ordering operators, `sort`, and the built-in `max` and `min` all order
through `compare`. When the left operand cannot order the pair, the right
operand's `compare` answers with the result negated, which is how
`10 > money` reaches Money's impl:

~~~goblin
type Money(amount) {
    impl Eq {
        func eq(self, other) {
            return Ord.compare(self, other) == 0
        }
    }
    impl Ord {
        func compare(self, other) {
            return self.amount - other
        }
    }
}

var m = Money(5)
print(m < 10)                           # true
print(10 > m)                           # true, answered by Money's compare
print(m == 5)                           # true, answered by Money's eq
~~~

Goblin cannot check the laws these traits rely on, so keep them yourself: a
custom `hash` must agree with `eq` (equal values hash alike), and `compare`
must be antisymmetric and transitive.

### Arithmetic

Each arithmetic operator has a trait of its own, so a type implements exactly
the operators it supports; a missing one raises the usual `cannot add Point`
TypeError. No operator is derived from another: `Add` and `Neg` do not give a
type `-`.

A binary trait requires the method for the value on the left. Its reflected
method (`radd`, `rsub`, ...) is optional and runs when the value is on the
right of an operand that does not know it, with that left operand as the
argument. Only a left operand reporting a type mismatch hands over to it, and
without the reflected method the left operand's error stands:

~~~goblin
type Vector(x, y) {
    impl Sub {
        func sub(self, other) {
            return Vector(self.x - other.x, self.y - other.y)
        }
    }
    impl Neg {
        func neg(self) {
            return Vector(-self.x, -self.y)
        }
    }
    impl Mul {
        func mul(self, k) {
            return Vector(self.x * k, self.y * k)
        }
        func rmul(self, k) {
            return Vector(k * self.x, k * self.y)
        }
    }
    impl Show {}
}

var v = Vector(1, 2)
print(v - Vector(1, 1))                 # Vector(x=0, y=1)
print(2 * v)                            # Vector(x=2, y=4)
print(-v)                               # Vector(x=-1, y=-2)
~~~

Built-in sequences follow the same rule, so `3 * "ab"` works like `"ab" * 3`.

### Truth, Iter and Index

`Truth.truth` decides every condition and `Bool(value)`; `!value` is always its
negation. `Iter.iter` returns a List that `for` visits. `Index.get` backs
`value[i]`; an impl without `set` is read-only, and assigning raises
`TypeError: Grid does not support index assignment`.

## Trait objects

A trait is an ordinary value: it prints as `<trait Eq>`, compares equal only to
itself, and its methods are values too (`var f = Named.greet`). The built-in
traits `Eq`, `Ord`, `Hashable`, `Show`, `Truth`, `Add`, `Sub`, `Mul`, `Div`, `Mod`, `Neg`, `Iter` and `Index` are
predeclared globals. Declaring a trait with one of these names shadows only the
name: `==` keeps using the built-in `Eq`.
