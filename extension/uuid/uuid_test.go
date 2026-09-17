package uuid

import (
	"errors"
	"testing"
	stduuid "uuid"

	goblintime "github.com/aisk/goblin/extension/time"
	"github.com/aisk/goblin/object"
)

func uuidFunction(t *testing.T, name string) *object.Function {
	t.Helper()
	modObj, err := Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	mod := modObj.(*object.Module)
	fn, ok := mod.Members[name].(*object.Function)
	if !ok {
		t.Fatalf("uuid module member %q is not a function", name)
	}
	return fn
}

func TestUUIDNew(t *testing.T) {
	for _, test := range []struct {
		name string
		args object.CallArgs
		want int
	}{
		{"default", object.CallArgs{}, 4},
		{"v4", object.CallArgs{Keyword: object.Kwargs{{Name: "version", Value: object.Integer(4)}}}, 4},
		{"v7", object.CallArgs{Positional: object.Args{object.Integer(7)}}, 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := uuidFunction(t, "new").Call(test.args)
			if err != nil {
				t.Fatalf("new() error = %v", err)
			}
			value, ok := got.(*UUID)
			if !ok {
				t.Fatalf("new() returned %T, want *UUID", got)
			}
			if value.version() != test.want {
				t.Fatalf("new() version = %d, want %d", value.version(), test.want)
			}
		})
	}
}

func TestUUIDNewRejectsUnsupportedVersion(t *testing.T) {
	for _, version := range []int64{1, 3, 5, 6, 8} {
		_, err := uuidFunction(t, "new").Call(object.CallArgs{Positional: object.Args{object.Integer(version)}})
		if err == nil || !errors.Is(err, object.ValueError) {
			t.Fatalf("new(%d) error = %v, want ValueError", version, err)
		}
	}
	_, err := uuidFunction(t, "new").Call(object.CallArgs{Positional: object.Args{object.String("4")}})
	if err == nil || !errors.Is(err, object.TypeError) {
		t.Fatalf("new(str) error = %v, want TypeError", err)
	}
}

func TestUUIDConstructorAcceptsUUIDStringAndBytes(t *testing.T) {
	want := stduuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	for _, value := range []object.Object{
		NewUUID(want),
		object.String("550E8400-E29B-41D4-A716-446655440000"),
		object.String("{550e8400-e29b-41d4-a716-446655440000}"),
		object.String("urn:uuid:550e8400-e29b-41d4-a716-446655440000"),
		object.String("550e8400e29b41d4a716446655440000"),
		object.Bytes(want[:]),
	} {
		got, err := uuidFunction(t, "UUID").Call(object.CallArgs{Keyword: object.Kwargs{{Name: "value", Value: value}}})
		if err != nil || got.(*UUID).Value != want {
			t.Fatalf("UUID(%v) = %v, %v; want %s, nil", value, got, err, want)
		}
	}
}

func TestUUIDConstructorRejectsInvalidValue(t *testing.T) {
	for _, value := range []object.Object{object.String("not-a-uuid"), object.Bytes("short")} {
		_, err := uuidFunction(t, "UUID").Call(object.CallArgs{Positional: object.Args{value}})
		if err == nil || !errors.Is(err, object.ParseError) {
			t.Fatalf("UUID(%v) error = %v, want ParseError", value, err)
		}
	}
	_, err := uuidFunction(t, "UUID").Call(object.CallArgs{Positional: object.Args{object.Integer(1)}})
	if err == nil || !errors.Is(err, object.TypeError) {
		t.Fatalf("UUID(int) error = %v, want TypeError", err)
	}
}

func TestUUIDConstants(t *testing.T) {
	modObj, _ := Execute()
	members := modObj.(*object.Module).Members
	if got := members["NIL"].(*UUID).String(); got != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("NIL = %s", got)
	}
	if got := members["MAX"].(*UUID).String(); got != "ffffffff-ffff-ffff-ffff-ffffffffffff" {
		t.Fatalf("MAX = %s", got)
	}
	if truthy, _ := members["NIL"].(*UUID).ToBool(); !truthy {
		t.Fatal("NIL should be truthy like every other UUID")
	}
}

func TestUUIDAttributes(t *testing.T) {
	id := NewUUID(stduuid.MustParse("550e8400-e29b-41d4-a716-446655440000"))
	checks := map[string]object.Object{
		"bytes":   object.Bytes(id.Value[:]),
		"urn":     object.String("urn:uuid:550e8400-e29b-41d4-a716-446655440000"),
		"version": object.Integer(4),
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
	if _, err := id.GetAttr("time"); err == nil || !errors.Is(err, object.ValueError) {
		t.Fatalf("v4.time error = %v, want ValueError", err)
	}
}

func TestUUIDTimeV7(t *testing.T) {
	// 0x017f22e279b0 ms is 2022-02-22T19:22:22Z, the RFC 9562 Appendix A.6 example.
	id := NewUUID(stduuid.MustParse("017f22e2-79b0-7cc3-98c4-dc0c0c07398f"))
	got, err := id.GetAttr("time")
	if err != nil {
		t.Fatalf("UUID.time error = %v", err)
	}
	if ms := got.(*goblintime.Time).Value.UnixMilli(); ms != 0x017f22e279b0 {
		t.Fatalf("UUID.time = %d ms, want %d", ms, int64(0x017f22e279b0))
	}
}

func TestUUIDTraits(t *testing.T) {
	a := NewUUID(stduuid.MustParse("00000000-0000-0000-0000-000000000001"))
	b := NewUUID(stduuid.MustParse("00000000-0000-0000-0000-000000000002"))
	call := func(trait *object.Trait, method string, args ...object.Object) object.Object {
		t.Helper()
		i, ok := trait.MethodIndex(method)
		if !ok {
			t.Fatalf("%s has no method %s", trait.Name, method)
		}
		got, err := trait.Invoke(i, object.CallArgs{Positional: args})
		if err != nil {
			t.Fatalf("%s.%s error = %v", trait.Name, method, err)
		}
		return got
	}

	if got := call(object.EqTrait, "eq", a, NewUUID(a.Value)); got != object.True {
		t.Fatalf("Eq.eq(a, copy of a) = %v, want true", got)
	}
	if got := call(object.EqTrait, "eq", a, object.String(a.String())); got != object.False {
		t.Fatalf("Eq.eq(a, str) = %v, want false", got)
	}
	if got := call(object.OrdTrait, "compare", a, b); got != object.Integer(-1) {
		t.Fatalf("Ord.compare(a, b) = %v, want -1", got)
	}
	if got := call(object.OrdTrait, "max", a, b); got != b {
		t.Fatalf("Ord.max(a, b) = %v, want b", got)
	}
	if call(object.HashableTrait, "hash", a) != call(object.HashableTrait, "hash", NewUUID(a.Value)) {
		t.Fatal("equal UUIDs must hash equal")
	}
	if got := call(object.ShowTrait, "show", a); got != object.String(a.String()) {
		t.Fatalf("Show.show(a) = %v", got)
	}
	if got := call(object.TruthTrait, "truth", NewUUID(stduuid.Nil())); got != object.True {
		t.Fatalf("Truth.truth(NIL) = %v, want true", got)
	}

	_, err := object.Compare(a, object.String(a.String()))
	if err == nil || !errors.Is(err, object.TypeError) {
		t.Fatalf("compare with str error = %v, want TypeError", err)
	}
	if _, err := object.Add(a, b); err == nil || !errors.Is(err, object.TypeError) {
		t.Fatalf("a + b error = %v, want TypeError", err)
	}
}
