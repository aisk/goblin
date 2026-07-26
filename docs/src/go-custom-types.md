# Custom object types

Every runtime value in Goblin is a Go implementation of object.Object. The
interface is the contract between a value and the interpreter or transpiler.
It includes these groups of methods:

| Group | Object methods |
| --- | --- |
| Display and conversion | String(), ToString(), Bool(), ToBool() |
| Comparison and operators | Compare(), Add(), Minus(), Multiply(), Divide(), And(), Or(), Not() |
| Collection protocols | Iter(), Index() |
| Members | GetAttr(), Attributes() |

Implement object.IndexSetter when a value supports item assignment, and
object.AttrSetter when it supports member assignment.

## Start with a Go struct

This excerpt shows the state, conversion, and member portion of a Counter
value. A complete implementation must also provide every remaining
object.Object method listed above. Unsupported operators should return the
standard TypeError rather than silently accepting an operation.

~~~go
type Counter struct {
    Value int64
}

func (c *Counter) String() string { return fmt.Sprintf("Counter(%d)", c.Value) }
func (c *Counter) ToString() (string, error) { return c.String(), nil }
func (c *Counter) Bool() bool { return c.Value != 0 }
func (c *Counter) ToBool() (bool, error) { return c.Bool(), nil }

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
    return nil, object.NewTypeError("cannot add Counter and %s", object.TypeName(other))
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
        return nil, object.NewTypeError("cannot add Vector and %s", object.TypeName(other))
    }
    return Vector{X: v.X + right.X, Y: v.Y + right.Y}, nil
}
~~~

Compare returns a negative value, zero, or a positive value. Iter returns a
slice of object.Object values. Index must verify that its index is an
object.Integer and return IndexError for an invalid position.

## Name types in messages with object.TypeName

Use object.TypeName(value), not %T, when an error mentions the type of a value.
A Go type name is an implementation detail that differs between the two
backends — a user-defined Point is \*interpreter.instance under `goblin run`
and \*main.Point in a compiled program — so %T makes the same program report
different messages depending on how it was executed. TypeName reports the
Goblin-level name. It uses the Go type name (without pointer and package
qualifier) for types outside object/, which is already the name a Goblin
program knows a custom type by; implement object.Named to report a different
one.

## Operators dispatch through package-level helpers

The `==`, `!=` and ordering operators do not call Equals and Compare on the
left operand directly: both backends go through object.Equals(a, b) and
object.Compare(a, b), which also give the right operand a chance to answer.
A custom type therefore only has to recognize the types it knows, and its
Compare is reached even when it appears on the right of `1 < value`. Report an
unordered pair with object.NewTypeError so the reflected attempt is made; any
other error propagates as-is.

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
