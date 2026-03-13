package extension

import (
	"strings"
	"testing"

	"github.com/aisk/goblin/object"
)

func mimeFunction(t *testing.T, name string) *object.Function {
	t.Helper()

	modObj, err := ExecuteMime()
	if err != nil {
		t.Fatalf("ExecuteMime() error = %v", err)
	}

	mod, ok := modObj.(*object.Module)
	if !ok {
		t.Fatalf("ExecuteMime() returned %T", modObj)
	}

	member, ok := mod.Members[name]
	if !ok {
		t.Fatalf("mime module missing %q", name)
	}

	fn, ok := member.(*object.Function)
	if !ok {
		t.Fatalf("mime module member %q is %T", name, member)
	}

	return fn
}

func TestMimeTypeByExtension(t *testing.T) {
	out, err := mimeFunction(t, "type_by_extension").Call(object.CallArgs{Positional: object.Args{object.String(".json")}})
	if err != nil {
		t.Fatalf("type_by_extension() error = %v", err)
	}

	s, ok := out.(object.String)
	if !ok {
		t.Fatalf("type_by_extension() returned %T", out)
	}
	if !strings.Contains(string(s), "application/json") {
		t.Fatalf("type_by_extension() = %q, want contains %q", string(s), "application/json")
	}
}

func TestMimeExtensionsByType(t *testing.T) {
	out, err := mimeFunction(t, "extensions_by_type").Call(object.CallArgs{Positional: object.Args{object.String("application/json")}})
	if err != nil {
		t.Fatalf("extensions_by_type() error = %v", err)
	}

	list, ok := out.(*object.List)
	if !ok {
		t.Fatalf("extensions_by_type() returned %T", out)
	}
	if len(list.Elements) == 0 {
		t.Fatalf("extensions_by_type() returned empty list")
	}
}

func TestMimeFunctionsRejectNonStringArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    object.Args
		wantErr string
	}{
		{name: "type_by_extension", args: object.Args{object.Integer(1)}, wantErr: "type_by_extension() argument must be a string"},
		{name: "extensions_by_type", args: object.Args{object.Integer(1)}, wantErr: "extensions_by_type() argument must be a string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mimeFunction(t, tt.name).Call(object.CallArgs{Positional: tt.args})
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
