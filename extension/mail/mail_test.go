package mail

import (
	"errors"
	"testing"

	"github.com/aisk/goblin/object"
)

func function(t *testing.T, name string) *object.Function {
	t.Helper()
	module, err := Execute()
	if err != nil {
		t.Fatal(err)
	}
	return module.(*object.Module).Members[name].(*object.Function)
}

func TestAddressConstructorAndParsing(t *testing.T) {
	constructed, err := function(t, "Address").Call(object.CallArgs{Positional: object.Args{
		object.String("Goblin"), object.String("goblin@example.com"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := function(t, "parse_address").Call(object.CallArgs{Positional: object.Args{object.String("Goblin <goblin@example.com>")}})
	if err != nil {
		t.Fatal(err)
	}
	equal, err := constructed.Equals(parsed)
	if err != nil || !equal {
		t.Fatalf("addresses differ: %v, %v", constructed, parsed)
	}
	name, _ := parsed.GetAttr("name")
	address, _ := parsed.GetAttr("address")
	if name != object.String("Goblin") || address != object.String("goblin@example.com") {
		t.Fatalf("parsed address = %v, %v", name, address)
	}
}

func TestParseAddressList(t *testing.T) {
	value, err := function(t, "parse_address_list").Call(object.CallArgs{Positional: object.Args{
		object.String("a@example.com, B <b@example.com>"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(value.(*object.List).Elements) != 2 {
		t.Fatalf("parse_address_list() = %v", value)
	}
}

func TestParseAddressError(t *testing.T) {
	_, err := function(t, "parse_address").Call(object.CallArgs{Positional: object.Args{object.String("not an address")}})
	if err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("parse_address() error = %v, want ParseError", err)
	}
}
