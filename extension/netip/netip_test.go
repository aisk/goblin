package netip

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func construct(t *testing.T, name, value string) object.Object {
	t.Helper()
	module, _ := Execute()
	result, err := module.(*object.Module).Members[name].(*object.Function).Call(object.CallArgs{Positional: object.Args{object.String(value)}})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAddr(t *testing.T) {
	addr := construct(t, "Addr", "192.168.1.10")
	private, _ := addr.GetAttr("is_private")
	if private != object.True {
		t.Fatalf("is_private = %v", private)
	}
	nextFn, _ := addr.GetAttr("next")
	next, err := nextFn.(*object.Function).Call(object.CallArgs{})
	nextText, _ := next.ToString()
	if err != nil || nextText != "192.168.1.11" {
		t.Fatalf("next() = %v, %v", next, err)
	}
}

func TestPrefix(t *testing.T) {
	prefix := construct(t, "Prefix", "192.168.1.5/24")
	maskedFn, _ := prefix.GetAttr("masked")
	masked, err := maskedFn.(*object.Function).Call(object.CallArgs{})
	maskedText, _ := masked.ToString()
	if err != nil || maskedText != "192.168.1.0/24" {
		t.Fatalf("masked() = %v, %v", masked, err)
	}
	containsFn, _ := prefix.GetAttr("contains")
	contains, err := containsFn.(*object.Function).Call(object.CallArgs{Positional: object.Args{construct(t, "Addr", "192.168.1.99")}})
	if err != nil || contains != object.True {
		t.Fatalf("contains() = %v, %v", contains, err)
	}
}
