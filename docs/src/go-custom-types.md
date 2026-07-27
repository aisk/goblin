# Custom object types

Every runtime value in Goblin is a Go implementation of object.Object. The
interface is the contract between a value and the interpreter or transpiler.
It includes these groups of methods:

| Group | Object methods |
| --- | --- |
| Display and conversion | ToString(), ToBool() |
| Comparison and operators | Equals(), Compare(), Add(), Minus(), Multiply(), Divide(), Not() |
| Reflected operators | RAdd(), RMinus(), RMultiply(), RDivide() |
| Collection protocols | Iter(), Index() |
| Members | GetAttr(), Attributes(), SetIndex(), SetAttr() |
| Identity | TypeName() |

The reflected operators are required, but most types have nothing to say
through them: embed object.NoReflectedOps and they all answer "not handled".

Types should also implement String() string, satisfying fmt.Stringer. It is
not part of the interface, but diagnostics and formatting reach it through
object.Inspect, which asserts fmt.Stringer and falls back to TypeName().
ToString is the failing counterpart that may run a user-defined __str.

Assignment is part of the interface too: SetIndex and SetAttr return a bool
saying whether the value accepts that form of assignment at all. A value that
accepts neither embeds object.NoAssignment and says nothing.

## Start with a Go struct

This excerpt shows the state, conversion, and member portion of a Counter
value. A complete implementation must also provide every remaining
object.Object method listed above. Unsupported operators should return the
standard TypeError rather than silently accepting an operation.

~~~go
type Counter struct {
    object.NoReflectedOps // nothing completes an operation from Counter's right
    object.NoAssignment   // counter[i] = x and counter.f = x are not accepted
    Value int64
}

func (c *Counter) TypeName() string { return "Counter" }
func (c *Counter) String() string { return fmt.Sprintf("Counter(%d)", c.Value) }
func (c *Counter) ToString() (string, error) { return c.String(), nil }
func (c *Counter) ToBool() (bool, error) { return c.Value != 0, nil }

func (c *Counter) GetAttr(name string) (object.Object, error) {
    switch name {
    case "value":
        return object.Integer(c.Value), nil
    case "increment":
        return &object.Function{Name: "increment", Fn: c.increment}, nil
    case "attributes":
        return object.AttributesFunction(c), nil
    default:
        return nil, object.NewAttributeError("Counter has no attribute '%s'", name)
    }
}

func (c *Counter) Attributes() []string {
    return []string{"attributes", "increment", "value"}
}

func (c *Counter) increment(args object.CallArgs) (object.Object, error) {
    if err := object.RequireNoKeyword("increment", args); err != nil {
        return nil, err
    }
    if len(args.Positional) != 0 {
        return nil, object.NewTypeError("increment() takes no arguments")
    }
    c.Value++
    return object.Integer(c.Value), nil
}
~~~

The receiver-bound object.Function is the key pattern: Goblin evaluates
counter.increment() by looking up increment and then calling the returned
function. It can safely mutate the Go receiver.

For example, a Counter that does not support addition should implement Add by
returning object.NewTypeError. Apply the same principle to the other
unsupported protocol methods.

~~~go
func (c *Counter) Add(other object.Object) (object.Object, error) {
    return nil, object.NewTypeError("cannot add Counter and %s", other.TypeName())
}
~~~

## Define protocol behavior deliberately

Return a useful result for supported operations and a TypeError for unsupported
ones. For example, a Vector can implement Add and Compare while a Counter may
only need display, truthiness, and members.

~~~go
func (v Vector) Add(other object.Object) (object.Object, error) {
    right, ok := other.(Vector)
    if !ok {
        return nil, object.NewTypeError("cannot add Vector and %s", other.TypeName())
    }
    return Vector{X: v.X + right.X, Y: v.Y + right.Y}, nil
}
~~~

Compare returns a negative value, zero, or a positive value. Iter returns a
slice of object.Object values. Index must verify that its index is an
object.Integer and return IndexError for an invalid position.

## Name types in messages with TypeName

Use value.TypeName(), not %T, when an error mentions the type of a value. A Go
type name is an implementation detail that differs between the two backends — a
user-defined Point is \*interpreter.instance under `goblin run` and \*main.Point
in a compiled program — so %T makes the same program report different messages
depending on how it was executed. TypeName returns the Goblin-level name, which
is why every type declares it:

~~~go
func (v Vector) TypeName() string { return "Vector" }
~~~

Embedding is not enough here: a type that embeds another for its defaults
inherits that type's name too, which is exactly the wrong answer.

## Operators dispatch through package-level helpers

The operators do not call Equals, Compare, Add and friends on the left operand
directly: both backends go through object.Equals(a, b), object.Compare(a, b),
object.Add(a, b), object.Minus(a, b), object.Multiply(a, b) and
object.Divide(a, b), which also give the right operand a chance to answer.
A custom type therefore only has to recognize the types it knows, and its
Compare is reached even when it appears on the right of `1 < value`. Report an
unfit pair with object.NewTypeError so the reflected attempt is made; any other
error propagates as-is. Equals follows the same rule with its own error return:
a TypeError from it means "not this type" and leaves `==` total, while any
other error fails the comparison.

Arithmetic reaches the right operand through RAdd, RMinus, RMultiply and
RDivide, the Go side of `__radd` and friends. Their argument is the LEFT
operand — RMinus(left) computes `left - receiver` — and the bool they return
reports whether the receiver handled this operand at all; returning false
leaves the left operand's error in place. Embed object.NoReflectedOps and
override only the operator a type completes from the right, the way
object.String and object.List override RMultiply to make `3 * "ab"` work:

~~~go
type Vector struct {
    object.NoReflectedOps
    X, Y float64
}

// `2 * v` — scaling reads the same in either operand order.
func (v Vector) RMultiply(left object.Object) (object.Object, bool, error) {
    n, ok := left.(object.Integer)
    if !ok {
        return nil, false, nil
    }
    return Vector{X: v.X * float64(n), Y: v.Y * float64(n)}, true, nil
}
~~~

For a complete reference implementation, read the existing runtime types in
object/, especially path.go, list.go, dict.go, bytes.go, and chan.go. They show
how to report errors consistently and how to expose methods through GetAttr.

## Expose a constructor

Most custom values need a constructor function added to a module or to the
built-ins map. The constructor validates arguments and returns the new value:

~~~go
var CounterConstructor = &object.Function{
    Name: "Counter",
    Fn: func(args object.CallArgs) (object.Object, error) {
        p := object.NewArgParser("Counter", args)
        start := p.IntOr("start", 0)
        if err := p.Finish(); err != nil {
            return nil, err
        }
        return &Counter{Value: int64(start)}, nil
    },
}
~~~

After placing this constructor in an object.Module Members map, Goblin code can
call it as module.Counter(start=10). See [Functions and
arguments](./go-functions-and-arguments.md) for the parser used here.
