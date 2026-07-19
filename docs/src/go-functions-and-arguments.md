# Functions and arguments

A Go function visible to Goblin is an object.Function. Its Fn callback receives
object.CallArgs and returns either a runtime value or an error.

~~~go
var Greet = &object.Function{
    Name: "greet",
    Fn: greet,
}

func greet(args object.CallArgs) (object.Object, error) {
    p := object.NewArgParser("greet", args)
    name := p.Str("name")
    excited := p.BoolOr("excited", false)
    if err := p.Finish(); err != nil {
        return nil, err
    }
    suffix := "."
    if excited {
        suffix = "!"
    }
    return object.String("Hello, " + string(name) + suffix), nil
}
~~~

This accepts both greet("Ada") and greet(name="Ada", excited=true). The order
of parser calls defines the positional argument order. Finish is mandatory: it
reports unconsumed positional arguments and unexpected keyword arguments.

## ArgParser accessors

NewArgParser accumulates the first argument error, letting the function extract
all of its inputs before checking once at Finish.

| Accessor | Meaning |
| --- | --- |
| Any(name) / AnyOr(name, default) | Required or optional arbitrary Object |
| Int, Float, Str, Bool | Required typed value |
| IntOr, FloatOr, StrOr, BoolOr | Optional typed value with a default |
| Number / NumberOr | Integer or Float |
| Float64 | Numeric value converted to Go float64 |
| Func | A Goblin function |
| OptionalAny | Value plus whether it was supplied |
| Rest | All remaining positional values |

Use OptionalAny when omitted and explicitly passing nil have different
meanings. Use Rest for an open-ended positional tail.

~~~go
func sum(args object.CallArgs) (object.Object, error) {
    p := object.NewArgParser("sum", args)
    values := p.Rest()
    if err := p.Finish(); err != nil {
        return nil, err
    }
    var total int64
    for _, value := range values {
        n, ok := value.(object.Integer)
        if !ok {
            return nil, object.NewTypeError("sum() values must be int")
        }
        total += int64(n)
    }
    return object.Integer(total), nil
}
~~~

## Other binding helpers

For a function that permits positional arguments only, call
object.RequireNoKeyword before checking the positional count. This is useful
for small no-options methods.

BindArguments is useful when an extension needs a declared parameter list plus
varargs or keyword captures. It binds positional and named values, detects
duplicates, and returns a map of parameter names to Object values.

~~~go
bound, err := object.BindArguments(
    "inspect",
    []string{"name"},
    "rest",
    "options",
    args,
)
if err != nil {
    return nil, err
}
name := bound["name"].(object.String)
rest := bound["rest"].(*object.List)
options := bound["options"].(*object.Dict)
~~~

Always return object.NewTypeError or another runtime error constructor for
user-facing failures. This preserves Goblin error handling and produces useful
tracebacks in both the interpreter and compiled executable.
