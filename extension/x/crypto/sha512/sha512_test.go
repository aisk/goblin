package sha512

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestSHA512Sums(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{{"sum", 64}, {"sum384", 48}, {"sum224", 28}, {"sum256", 32}}
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
