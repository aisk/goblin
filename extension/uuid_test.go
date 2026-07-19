package extension

import (
	"testing"

	"github.com/aisk/goblin/object"
	googleuuid "github.com/google/uuid"
)

func uuidFunction(t *testing.T, name string) *object.Function {
	t.Helper()
	modObj, err := ExecuteUUID()
	if err != nil {
		t.Fatalf("ExecuteUUID() error = %v", err)
	}
	mod := modObj.(*object.Module)
	fn, ok := mod.Members[name].(*object.Function)
	if !ok {
		t.Fatalf("uuid module member %q is not a function", name)
	}
	return fn
}

func TestUUIDNew(t *testing.T) {
	got, err := uuidFunction(t, "new").Call(object.CallArgs{})
	if err != nil {
		t.Fatalf("new() error = %v", err)
	}
	value, ok := got.(*UUID)
	if !ok {
		t.Fatalf("new() returned %T, want *UUID", got)
	}
	if err := googleuuid.Validate(value.String()); err != nil {
		t.Fatalf("new() returned invalid UUID %q: %v", value, err)
	}
}

func TestUUIDParseAndValidate(t *testing.T) {
	const input = "550E8400-E29B-41D4-A716-446655440000"
	got, err := uuidFunction(t, "parse").Call(object.CallArgs{Positional: object.Args{object.String(input)}})
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	parsed, ok := got.(*UUID)
	if !ok || parsed.String() != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("parse() = %v, want canonical UUID", got)
	}

	valid, err := uuidFunction(t, "validate").Call(object.CallArgs{Positional: object.Args{object.String(input)}})
	if err != nil {
		t.Fatalf("validate() error = %v", err)
	}
	if valid != object.True {
		t.Fatalf("validate(valid UUID) = %v, want true", valid)
	}
	valid, err = uuidFunction(t, "validate").Call(object.CallArgs{Positional: object.Args{parsed}})
	if err != nil {
		t.Fatalf("validate(UUID) error = %v", err)
	}
	if valid != object.True {
		t.Fatalf("validate(UUID) = %v, want true", valid)
	}

	invalid, err := uuidFunction(t, "validate").Call(object.CallArgs{Positional: object.Args{object.String("not-a-uuid")}})
	if err != nil {
		t.Fatalf("validate() error = %v", err)
	}
	if invalid != object.False {
		t.Fatalf("validate(invalid UUID) = %v, want false", invalid)
	}
}

func TestUUIDParseRejectsInvalidValue(t *testing.T) {
	_, err := uuidFunction(t, "parse").Call(object.CallArgs{Positional: object.Args{object.String("not-a-uuid")}})
	if err == nil {
		t.Fatal("parse() succeeded for an invalid UUID")
	}
}
