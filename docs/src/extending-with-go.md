# Extending Goblin with Go

Goblin is implemented in Go, and its runtime values are Go values. This makes
the standard library extension model straightforward: write Go code that
implements the runtime contracts, expose it as a module or built-in function,
then make both Goblin execution backends aware of it.

This chapter is for contributors extending the Goblin repository, not for
ordinary Goblin programs. The extension code imports the repository packages:

~~~go
import (
    "github.com/aisk/goblin/object"
)
~~~

## The runtime boundary

Every Goblin value implements object.Object. Integer, Float, String, List,
Dict, module values, functions, and user-defined Goblin types all appear to
the Go runtime through this interface. It supplies conversion, comparison,
operators, iteration, indexing, and attribute access.

This single interface is why a Go extension can participate naturally in
Goblin expressions. A custom value can decide how it prints, whether it is
truthy, what an attribute lookup returns, and which operators are valid.
[Custom object types](./go-custom-types.md) explains the contract in detail.

## Adding a function or module

A Go-callable Goblin function has this shape:

~~~go
func(args object.CallArgs) (object.Object, error)
~~~

Place related functions in an extension package and return them from an
object.Module:

~~~go
func Execute() (object.Object, error) {
    return &object.Module{
        Members: map[string]object.Object{
            "greet": &object.Function{Name: "greet", Fn: greet},
        },
    }, nil
}
~~~

A new built-in module lives in its own Go package, extension/<module name>,
and exposes that function as Execute. Register it once, in the table in
extension/stdlib/stdlib.go. goblin run loads modules from that table, and
goblin build-exe uses it to validate the import before emitting a direct
import of the module's package, so a compiled program links only the modules
it imports. A test in extension/stdlib checks that every registered executor
really is extension/<module name>.Execute.

Add a focused Go test for the extension and a Goblin example when its user
visible behavior needs end-to-end coverage.

## Choosing an extension shape

Use a plain object.Function for a stateless operation such as a conversion or
utility. Use object.Module for a named collection of functions and constants.
Use a custom object.Object type when the feature has its own state, methods,
or protocol behavior, such as a Path, HTTP response, file, or time value.

The next chapters show custom values and safe argument parsing.
