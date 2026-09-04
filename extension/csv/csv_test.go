package csv

import (
	"bytes"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
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
	if !modtest.Equal(decoded, records) {
		t.Fatalf("round trip = %v", decoded)
	}
}

func TestCSVWriteAllToDest(t *testing.T) {
	records := &object.List{Elements: []object.Object{
		&object.List{Elements: []object.Object{object.String("name"), object.String("score")}},
		&object.List{Elements: []object.Object{object.String("Ada"), object.String("10")}},
	}}

	var buffer bytes.Buffer
	result, err := csvWriteAll(object.CallArgs{
		Positional: object.Args{records},
		Keyword:    object.Kwargs{{Name: "dest", Value: modtest.DestRecorder(&buffer)}},
	})
	if err != nil {
		t.Fatalf("write_all() error = %v", err)
	}
	if _, ok := result.(object.Unit); !ok {
		t.Fatalf("write_all(dest=...) = %v, want nil", result)
	}
	if got, want := buffer.String(), "name,score\nAda,10\n"; got != want {
		t.Fatalf("dest received %q, want %q", got, want)
	}
}
