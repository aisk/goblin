# Protocols

`impl` connects a type to the language's syntax; everything else is an
ordinary method.

~~~goblin
type Money(amount) {
    impl Eq {}                                  # structural
    impl Ord {
        func compare(self, other) { return self.amount - other.amount }
    }
    impl Show {
        func show(self) { return "${self.amount}" }
    }
    func to_json(self) { return self.amount }   # a method the library asks for
}

var prices = [Money(5), Money(2), Money(9)]
print(prices.sort())                    # [$2, $5, $9]
print(Money(2) < Money(5), max(prices)) # true $9
~~~

A protocol is one of thirteen fixed names. Each connects a type to a piece of
syntax or a built-in conversion: implement `Ord` and `<`, `sort`, `max` and
`compare` work on the type; implement `Add` and `+` does. The set is closed: a
program cannot declare a protocol, and neither can a library.

## Implementing a protocol

An impl block sits inside a type body, among the ordinary methods, in any
order: `impl Name { ... }`. A type has at most one impl per protocol. The
block defines the protocol's methods, each with `self` first and a fixed
parameter list: no defaults, `*args` or `**kwargs`.

The name after `impl` is never looked up as a variable. The thirteen names are
not globals and have no value (`print(Eq)` is `undefined identifier: Eq`), so a
program may use them for its own things, and `impl Show` still means the
protocol in a module that declares `type Show(...)`. Any other name is an
error when the type is defined:

~~~text
impl Shape for Rect: Shape is not a protocol (protocols: Eq, Ord, Hashable,
Show, Truth, Add, Sub, Mul, Div, Mod, Neg, Iter, Index); other methods go
directly in the type body
~~~

Protocol methods never become attributes of the instance: an ordinary method
or field may reuse a protocol method's name, so a type can have a `get` method
of its own beside `impl Index { func get(self, i) { ... } }`, and `p.add(x)`
calls the ordinary `add`, not `+`. A protocol method is reached through its
syntax. `compare(a, b)` and `hash(x)` are the global functions for the two
that have none.

| Protocol | Required | Optional | Enables | Empty impl |
| --- | --- | --- | --- | --- |
| `Eq` | `eq(self, other)` | | `==`, `!=` | structural |
| `Ord` (needs `Eq`) | `compare(self, other)` | | `<`, `<=`, `>`, `>=`, `sort`, `max`, `min`, `compare` | structural |
| `Hashable` (needs `Eq`) | `hash(self)` | | dict keys, `hash` | structural |
| `Show` | `show(self)` | | `print`, `String`, interpolation | structural |
| `Truth` | `truth(self)` | | `if`, `while`, `&&`, `\|\|`, `!`, `Bool` | not allowed |
| `Add` `Sub` `Mul` `Div` `Mod` | `add` `sub` `mul` `div` `mod` `(self, other)` | `radd` `rsub` `rmul` `rdiv` `rmod` `(self, other)` | `+ - * / %` | not allowed |
| `Neg` | `neg(self)` | | unary `-` | not allowed |
| `Iter` | `iter(self)` | | `for x in value`, `List(value)` | not allowed |
| `Index` | `get(self, index)` | `set(self, index, value)` | `value[i]`, `value[i] = x` | not allowed |

One required method gives a whole family that cannot disagree: `compare` is
the only ordering method, so `<`, `sort` and `max` always agree, and `!=` is
always the negation of `==`. A method the protocol does not list is an error
(`impl Ord for Point: Ord has no method 'lt'`), as are a missing required
method (`impl Truth for Flag is missing method 'truth'`), a wrong parameter
count, and a missing dependency (`impl Ord for Point requires impl Eq`). All of
them are reported when the type is defined, before any value exists.

`compare` and `hash` must return an Int, `show` a String, and `eq` and
`truth` a Bool; anything else raises
`TypeError: Point.show must return String, got Int` when the method runs.

Without an impl, the defaults are: `==` is identity, printing shows
`<Point@0x...>`, a value is truthy, and ordering, arithmetic, iteration,
indexing and use as a dict key raise TypeError, which says what to add:
`cannot hash Point (add 'impl Hashable' to type Point)`.

## Structural implementations

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

## Eq and Ord

`Ord` depends on `Eq`, and like every dependency the type implements it
explicitly. A structural `Ord` pairs with `impl Eq {}`; a custom `compare`
usually pairs with an `eq` that asks it, as `Money` does below, so that `==`
agrees with the ordering.

Equality never raises over a type mismatch. `a == b` asks the left operand's
`eq`, then the right one's, and falls back to identity; a TypeError raised
inside an `eq` means "unequal", so `money == nil` stays false for an `eq`
written only for numbers. Any other error propagates. `!=` is always the
negation of `==`.

The ordering operators, `sort`, and the built-in `compare`, `max` and `min`
all order through `compare`. When the left operand cannot order the pair, the right
operand's `compare` answers with the result negated, which is how
`10 > money` reaches Money's impl:

~~~goblin
type Money(amount) {
    impl Eq {
        func eq(self, other) {
            return compare(self, other) == 0
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

Goblin cannot check the laws these protocols rely on, so keep them yourself: a
custom `hash` must agree with `eq` (equal values hash alike), and `compare`
must be antisymmetric and transitive.

## Arithmetic

Each arithmetic operator has a protocol of its own, so a type implements exactly
the operators it supports; a missing one raises the usual `cannot add Point`
TypeError. No operator is derived from another: `Add` and `Neg` do not give a
type `-`.

A binary protocol requires the method for the value on the left. Its reflected
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

## Truth, Iter and Index

A `truth` method decides every condition and `Bool(value)`; `!value` is always its
negation. An `iter` method returns any iterable value — a List, a String, a
file, another value with an Iter impl — and `for` visits what iterating that
value visits. A lazy value stays lazy: a type whose `iter` returns an open file
reads it line by line. An `Index` impl's `get` backs
`value[i]`; an impl without `set` is read-only, and assigning raises
`TypeError: Grid does not support index assignment`.

## Methods the library asks for

Library functions that need something from a value ask for an ordinary method
by name, not for a protocol: `read(size)` for something readable,
`write(data)` for something writable, and `to_json()` for a value [json](./module-json.md) should encode. Define them
directly in the type body; there is nothing to import or implement.
