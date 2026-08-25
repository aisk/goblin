package unicode

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestUnicodeClassify(t *testing.T) {
	if got := modtest.CallModule(t, Execute, "is_letter", object.CallArgs{Positional: object.Args{object.String("界")}}); got != object.True {
		t.Fatalf("is_letter() = %v", got)
	}
	if got := modtest.CallModule(t, Execute, "to_upper", object.CallArgs{Positional: object.Args{object.String("g")}}); got != object.String("G") {
		t.Fatalf("to_upper() = %v", got)
	}
}
