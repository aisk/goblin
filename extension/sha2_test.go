package extension

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestSHA2Sums(t *testing.T) {
	tests := []struct {
		call func(object.CallArgs) (object.Object, error)
		size int
	}{
		{sha256Sum224, 28}, {sha256Sum256, 32}, {sha512Sum384, 48},
		{sha512Sum512, 64}, {sha512Sum512224, 28}, {sha512Sum512256, 32},
	}
	for _, test := range tests {
		value, err := test.call(object.CallArgs{Positional: object.Args{object.String("goblin")}})
		if err != nil {
			t.Fatal(err)
		}
		if len(value.(object.Bytes)) != test.size {
			t.Fatalf("digest size = %d, want %d", len(value.(object.Bytes)), test.size)
		}
	}
}
