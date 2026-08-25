package sha256

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestSHA256Sums(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{{"sum", 32}, {"sum224", 28}}
	for _, test := range tests {
		module, err := Execute()
		if err != nil {
			t.Fatal(err)
		}
		function := module.(*object.Module).Members[test.name].(*object.Function)
		value, err := function.Call(object.CallArgs{Positional: object.Args{object.String("goblin")}})
		if err != nil {
			t.Fatal(err)
		}
		if len(value.(object.Bytes)) != test.size {
			t.Fatalf("digest size = %d, want %d", len(value.(object.Bytes)), test.size)
		}
	}
}

func TestSHA256Hex(t *testing.T) {
	module, err := Execute()
	if err != nil {
		t.Fatal(err)
	}
	value, err := module.(*object.Module).Members["hex"].(*object.Function).Call(object.CallArgs{Positional: object.Args{object.String("goblin")}})
	if err != nil {
		t.Fatal(err)
	}
	if value != object.String("f59ddf918f384a1b7e1d1011c49c3f3fd38421fc3ed3d90dfaa9bb1633325478") {
		t.Fatalf("hex() = %v", value)
	}
}
