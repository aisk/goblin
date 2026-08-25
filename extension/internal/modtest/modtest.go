// Package modtest holds the helpers shared by the stdlib module package
// tests. It is imported only from _test files.
package modtest

import (
	"bytes"
	"testing"

	"github.com/aisk/goblin/object"
)

// CallModule executes a stdlib module, looks up the named function member,
// and calls it with the given positional and/or keyword arguments, failing
// the test on any error.
func CallModule(t *testing.T, execute object.ModuleExecutor, name string, call object.CallArgs) object.Object {
	t.Helper()
	module, err := execute()
	if err != nil {
		t.Fatalf("execute module: %v", err)
	}
	fn, ok := module.(*object.Module).Members[name].(*object.Function)
	if !ok {
		t.Fatalf("module member %q is not a function", name)
	}
	value, err := fn.Call(call)
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return value
}

// Equal answers the tests' equality checks. Comparing two built-in values
// never fails, so an error here can only be a bug in the test setup.
func Equal(a, b object.Object) bool {
	eq, err := object.Equals(a, b)
	if err != nil {
		panic(err)
	}
	return eq
}

// DestRecorder is a duck-typed writer handed to the dest keyword: any object
// with a write(data) method qualifies, a Module is just the cheapest way to
// build one in tests.
func DestRecorder(buffer *bytes.Buffer) object.Object {
	return &object.Module{Name: "recorder", Members: map[string]object.Object{
		"write": &object.Function{Name: "write", Fn: func(args object.CallArgs) (object.Object, error) {
			chunk := args.Positional[0].(object.Bytes)
			buffer.Write(chunk)
			return object.Integer(len(chunk)), nil
		}},
	}}
}
