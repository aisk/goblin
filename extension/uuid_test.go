package extension

import (
	"errors"
	"strconv"
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
	if value.Value.Version() != 4 {
		t.Fatalf("new() version = %d, want 4", value.Value.Version())
	}
}

func TestUUIDNewVersions(t *testing.T) {
	for _, version := range []int64{1, 4, 6, 7} {
		t.Run(strconv.FormatInt(version, 10), func(t *testing.T) {
			got, err := uuidFunction(t, "new").Call(object.CallArgs{Keyword: object.Kwargs{
				"version": object.Integer(version),
			}})
			if err != nil {
				t.Fatalf("new(version=%d) error = %v", version, err)
			}
			if got.(*UUID).Value.Version() != googleuuid.Version(version) {
				t.Fatalf("new(version=%d) returned version %d", version, got.(*UUID).Value.Version())
			}
		})
	}
}

func TestUUIDNewNameBased(t *testing.T) {
	for _, test := range []struct {
		version int64
		data    object.Object
		want    googleuuid.UUID
	}{
		{3, object.String("example.com"), googleuuid.NewMD5(googleuuid.NameSpaceDNS, []byte("example.com"))},
		{5, object.Bytes("example.com"), googleuuid.NewSHA1(googleuuid.NameSpaceDNS, []byte("example.com"))},
	} {
		got, err := uuidFunction(t, "new").Call(object.CallArgs{Keyword: object.Kwargs{
			"version":   object.Integer(test.version),
			"namespace": NewUUID(googleuuid.NameSpaceDNS),
			"data":      test.data,
		}})
		if err != nil {
			t.Fatalf("new(version=%d) error = %v", test.version, err)
		}
		if got.(*UUID).Value != test.want {
			t.Fatalf("new(version=%d) = %s, want %s", test.version, got, test.want)
		}
	}
}

func TestUUIDNewRejectsInvalidArgumentCombinations(t *testing.T) {
	tests := []struct {
		name string
		args object.CallArgs
		kind *object.Error
	}{
		{"unsupported version", object.CallArgs{Keyword: object.Kwargs{"version": object.Integer(2)}}, object.ValueError},
		{"missing namespace", object.CallArgs{Keyword: object.Kwargs{"version": object.Integer(5), "data": object.String("x")}}, object.TypeError},
		{"missing data", object.CallArgs{Keyword: object.Kwargs{"version": object.Integer(5), "namespace": NewUUID(googleuuid.NameSpaceDNS)}}, object.TypeError},
		{"wrong namespace type", object.CallArgs{Keyword: object.Kwargs{"version": object.Integer(5), "namespace": object.String("dns"), "data": object.String("x")}}, object.TypeError},
		{"wrong data type", object.CallArgs{Keyword: object.Kwargs{"version": object.Integer(5), "namespace": NewUUID(googleuuid.NameSpaceDNS), "data": object.Integer(1)}}, object.TypeError},
		{"data with random version", object.CallArgs{Keyword: object.Kwargs{"version": object.Integer(4), "data": object.String("x")}}, object.TypeError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := uuidFunction(t, "new").Call(test.args)
			if err == nil || !errors.Is(err, test.kind) {
				t.Fatalf("new() error = %v, want %s", err, test.kind)
			}
		})
	}
}

func TestUUIDConstructAndValidate(t *testing.T) {
	const input = "550E8400-E29B-41D4-A716-446655440000"
	got, err := uuidFunction(t, "UUID").Call(object.CallArgs{Positional: object.Args{object.String(input)}})
	if err != nil {
		t.Fatalf("UUID() error = %v", err)
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
	invalid, err := uuidFunction(t, "validate").Call(object.CallArgs{Positional: object.Args{object.String("not-a-uuid")}})
	if err != nil {
		t.Fatalf("validate() error = %v", err)
	}
	if invalid != object.False {
		t.Fatalf("validate(invalid UUID) = %v, want false", invalid)
	}
}

func TestUUIDConstructorAcceptsUUIDStringAndBytes(t *testing.T) {
	want := googleuuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	for _, value := range []object.Object{
		NewUUID(want),
		object.String(want.String()),
		object.Bytes(want[:]),
	} {
		got, err := uuidFunction(t, "UUID").Call(object.CallArgs{Keyword: object.Kwargs{"value": value}})
		if err != nil || got.(*UUID).Value != want {
			t.Fatalf("UUID(%s) = %v, %v; want %s, nil", value.TypeName(), got, err, want)
		}
	}
}

func TestUUIDConstructorRejectsInvalidValue(t *testing.T) {
	_, err := uuidFunction(t, "UUID").Call(object.CallArgs{Positional: object.Args{object.String("not-a-uuid")}})
	if err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("UUID() error = %v, want ParseError", err)
	}
	_, err = uuidFunction(t, "UUID").Call(object.CallArgs{Positional: object.Args{object.Bytes("short")}})
	if err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("UUID(short Bytes) error = %v, want ParseError", err)
	}
}

func TestUUIDFunctionsAcceptKeywords(t *testing.T) {
	const input = "550e8400-e29b-41d4-a716-446655440000"
	if _, err := uuidFunction(t, "UUID").Call(object.CallArgs{Keyword: object.Kwargs{"value": object.String(input)}}); err != nil {
		t.Fatalf("UUID(value=...) error = %v", err)
	}
	got, err := uuidFunction(t, "validate").Call(object.CallArgs{Keyword: object.Kwargs{"value": object.String(input)}})
	if err != nil || got != object.True {
		t.Fatalf("validate(value=...) = %v, %v; want true, nil", got, err)
	}
}

func TestUUIDAttributes(t *testing.T) {
	id := NewUUID(googleuuid.MustParse("550e8400-e29b-41d4-a716-446655440000"))
	checks := map[string]object.Object{
		"bytes":   object.Bytes(id.Value[:]),
		"urn":     object.String("urn:uuid:550e8400-e29b-41d4-a716-446655440000"),
		"version": object.Integer(4),
		"variant": object.String("RFC4122"),
	}
	for name, want := range checks {
		got, err := id.GetAttr(name)
		if err != nil {
			t.Fatalf("UUID.%s error = %v", name, err)
		}
		equal, err := object.Equals(got, want)
		if err != nil || !equal {
			t.Fatalf("UUID.%s = %v, want %v", name, got, want)
		}
	}
}

func TestUUIDVersionSpecificAttributes(t *testing.T) {
	v1, err := uuidFunction(t, "new").Call(object.CallArgs{Keyword: object.Kwargs{"version": object.Integer(1)}})
	if err != nil {
		t.Fatalf("new(version=1) error = %v", err)
	}
	for _, name := range []string{"time", "clock_sequence", "node"} {
		if _, err := v1.(*UUID).GetAttr(name); err != nil {
			t.Fatalf("v1.%s error = %v", name, err)
		}
	}

	v4 := NewUUID(googleuuid.New())
	for _, name := range []string{"time", "clock_sequence", "node"} {
		if _, err := v4.GetAttr(name); err == nil || !errors.Is(err, object.ValueError) {
			t.Fatalf("v4.%s error = %v, want ValueError", name, err)
		}
	}
}

func TestUUIDValidateRequiresString(t *testing.T) {
	_, err := uuidFunction(t, "validate").Call(object.CallArgs{Positional: object.Args{NewUUID(googleuuid.NameSpaceDNS)}})
	if err == nil || !errors.Is(err, object.TypeError) {
		t.Fatalf("validate(UUID) error = %v, want TypeError", err)
	}
}
