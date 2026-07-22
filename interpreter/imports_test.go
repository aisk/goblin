package interpreter

import (
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
)

func TestResolveImportOsCached(t *testing.T) {
	// The registry keys modules by import path only. argv is closed over when
	// "os" is first loaded; later resolveImport calls with a different argv
	// still return that same module. Run and Session keep one argv per
	// registry, so this is the intended contract.
	reg := object.NewRegistry()
	argv := []string{"app.goblin", "foo"}
	imp := &ast.Import{Name: "os", Path: "os"}

	first, err := resolveImport(imp, ".", reg, argv)
	if err != nil {
		t.Fatalf("first resolveImport() error = %v", err)
	}
	second, err := resolveImport(imp, ".", reg, argv)
	if err != nil {
		t.Fatalf("second resolveImport() error = %v", err)
	}
	if first != second {
		t.Fatal("expected cached os module from registry")
	}

	// Different argv must not replace the cached module or its snapshot.
	other, err := resolveImport(imp, ".", reg, []string{"other.goblin", "bar"})
	if err != nil {
		t.Fatalf("resolveImport with other argv: %v", err)
	}
	if other != first {
		t.Fatal("expected same cached os module when argv differs")
	}

	mod, ok := first.(*object.Module)
	if !ok {
		t.Fatalf("os import = %T, want *object.Module", first)
	}
	fn, ok := mod.Members["argv"].(*object.Function)
	if !ok {
		t.Fatalf("argv = %T, want *object.Function", mod.Members["argv"])
	}
	got, err := fn.Call(object.CallArgs{})
	if err != nil {
		t.Fatalf("argv() error = %v", err)
	}
	list, ok := got.(*object.List)
	if !ok {
		t.Fatalf("argv() = %T, want *object.List", got)
	}
	want := []string{"app.goblin", "foo"}
	if len(list.Elements) != len(want) {
		t.Fatalf("argv() size = %d, want %d (first-load snapshot)", len(list.Elements), len(want))
	}
	for i, elem := range list.Elements {
		s, ok := elem.(object.String)
		if !ok {
			t.Fatalf("argv()[%d] is %T, want object.String", i, elem)
		}
		if string(s) != want[i] {
			t.Fatalf("argv()[%d] = %q, want %q (first-load snapshot)", i, s, want[i])
		}
	}
}
