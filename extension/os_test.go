package extension

import (
	"os"
	"reflect"
	"testing"

	"github.com/aisk/goblin/extension/fs"
	"github.com/aisk/goblin/object"
)

func argvFunction(t *testing.T, module object.Object) *object.Function {
	t.Helper()
	mod, ok := module.(*object.Module)
	if !ok {
		t.Fatalf("os module = %T, want *object.Module", module)
	}
	fn, ok := mod.Members["argv"].(*object.Function)
	if !ok {
		t.Fatalf("os.argv = %T, want *object.Function", mod.Members["argv"])
	}
	return fn
}

func TestOsTempFunctionsMirrorGoResources(t *testing.T) {
	dirValue, err := mkdirTemp(object.CallArgs{Keyword: object.Kwargs{"pattern": object.String("goblin-*")}})
	if err != nil {
		t.Fatal(err)
	}
	dir := string(dirValue.(object.String))
	defer os.Remove(dir)

	fileValue, err := createTemp(object.CallArgs{Keyword: object.Kwargs{
		"dir": object.String(dir), "pattern": object.String("data-*"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	file := fileValue.(*fs.File)
	defer os.Remove(file.Name)
	if _, err := file.Write(object.CallArgs{Positional: object.Args{object.String("goblin")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Close(object.CallArgs{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file.Name)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "goblin" {
		t.Fatalf("temporary file = %q", data)
	}
}

func callArgv(t *testing.T, fn *object.Function) (*object.List, []string) {
	t.Helper()
	value, err := fn.Call(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	list, ok := value.(*object.List)
	if !ok {
		t.Fatalf("os.argv() = %T, want *object.List", value)
	}
	strings := make([]string, len(list.Elements))
	for i, elem := range list.Elements {
		s, ok := elem.(object.String)
		if !ok {
			t.Fatalf("os.argv()[%d] = %T, want object.String", i, elem)
		}
		strings[i] = string(s)
	}
	return list, strings
}

func TestOsArgv(t *testing.T) {
	module, err := ExecuteOs()
	if err != nil {
		t.Fatal(err)
	}
	fn := argvFunction(t, module)
	_, got := callArgv(t, fn)
	if !reflect.DeepEqual(got, os.Args) {
		t.Fatalf("os.argv() = %q, want %q", got, os.Args)
	}

	if _, err := fn.Call(object.CallArgs{Positional: object.Args{object.Integer(1)}}); err == nil {
		t.Fatal("os.argv(1) should fail")
	}
	if _, err := fn.Call(object.CallArgs{Keyword: object.Kwargs{"x": object.Integer(1)}}); err == nil {
		t.Fatal("os.argv(x=1) should fail")
	}
}

func TestOsArgvFrozenAndFresh(t *testing.T) {
	input := []string{"script.goblin", "foo"}
	module, err := ExecuteOsWithFrozenArgs(input)
	if err != nil {
		t.Fatal(err)
	}
	fn := argvFunction(t, module)

	first, got := callArgv(t, fn)
	if want := []string{"script.goblin", "foo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("os.argv() = %q, want %q", got, want)
	}

	// Neither the caller's input nor a previously returned Goblin list may
	// mutate the frozen process arguments.
	input[1] = "changed input"
	first.Elements[1] = object.String("changed result")
	if _, got := callArgv(t, fn); !reflect.DeepEqual(got, []string{"script.goblin", "foo"}) {
		t.Fatalf("second os.argv() = %q, want a fresh copy of the frozen arguments", got)
	}
}
