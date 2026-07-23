package extension

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestCSVRoundTrip(t *testing.T) {
	records := &object.List{Elements: []object.Object{
		&object.List{Elements: []object.Object{object.String("name"), object.String("note")}},
		&object.List{Elements: []object.Object{object.String("Goblin"), object.String("a,b")}},
	}}
	encoded, err := csvWriteAll(object.CallArgs{Positional: object.Args{records}})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := csvReadAll(object.CallArgs{Positional: object.Args{encoded}})
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.Equals(records) {
		t.Fatalf("round trip = %v", decoded)
	}
}
