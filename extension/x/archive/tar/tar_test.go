package tar

import (
	"bytes"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestTarRoundTrip(t *testing.T) {
	files := object.NewDict()
	_ = files.Set(object.String("a.txt"), object.String("alpha"))
	_ = files.Set(object.String("dir/b.bin"), object.NewBytes([]byte{0, 1, 2}))
	archive, err := tarWriteAll(object.CallArgs{Positional: object.Args{files}})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := tarReadAll(object.CallArgs{Positional: object.Args{archive}})
	if err != nil {
		t.Fatal(err)
	}
	result := decoded.(*object.Dict)
	alpha, _, err := result.Get(object.String("a.txt"))
	if err != nil || string(alpha.(object.Bytes)) != "alpha" {
		t.Fatalf("text entry = %v, %v", alpha, err)
	}
	binary, _, err := result.Get(object.String("dir/b.bin"))
	if err != nil || !modtest.Equal(binary, object.NewBytes([]byte{0, 1, 2})) {
		t.Fatalf("binary entry = %v, %v", binary, err)
	}
}

func TestTarWriteAllToDest(t *testing.T) {
	files := object.NewDict()
	if err := files.Set(object.String("a.txt"), object.String("alpha")); err != nil {
		t.Fatal(err)
	}

	var buffer bytes.Buffer
	result, err := tarWriteAll(object.CallArgs{
		Positional: object.Args{files},
		Keyword:    object.Kwargs{{Name: "dest", Value: modtest.DestRecorder(&buffer)}},
	})
	if err != nil {
		t.Fatalf("write_all() error = %v", err)
	}
	if _, ok := result.(object.Unit); !ok {
		t.Fatalf("write_all(dest=...) = %v, want nil", result)
	}

	restored, err := tarReadAll(object.CallArgs{Positional: object.Args{object.NewBytes(buffer.Bytes())}})
	if err != nil {
		t.Fatalf("read_all() error = %v", err)
	}
	content, found, err := restored.(*object.Dict).Get(object.String("a.txt"))
	if err != nil || !found {
		t.Fatalf("Get() = %v, %v", found, err)
	}
	if got := string(content.(object.Bytes)); got != "alpha" {
		t.Fatalf("round trip = %q, want alpha", got)
	}
}
