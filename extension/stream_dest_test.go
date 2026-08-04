package extension

import (
	"bytes"
	"testing"

	"github.com/aisk/goblin/object"
)

// destRecorder is a duck-typed writer handed to the dest keyword: any object
// with a write(data) method qualifies, a Module is just the cheapest way to
// build one in tests.
func destRecorder(buffer *bytes.Buffer) object.Object {
	return &object.Module{Name: "recorder", Members: map[string]object.Object{
		"write": &object.Function{Name: "write", Fn: func(args object.CallArgs) (object.Object, error) {
			chunk := args.Positional[0].(object.Bytes)
			buffer.Write(chunk)
			return object.Integer(len(chunk)), nil
		}},
	}}
}

func TestCSVWriteAllToDest(t *testing.T) {
	records := &object.List{Elements: []object.Object{
		&object.List{Elements: []object.Object{object.String("name"), object.String("score")}},
		&object.List{Elements: []object.Object{object.String("Ada"), object.String("10")}},
	}}

	var buffer bytes.Buffer
	result, err := csvWriteAll(object.CallArgs{
		Positional: object.Args{records},
		Keyword:    map[string]object.Object{"dest": destRecorder(&buffer)},
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

func TestGzipCompressToDest(t *testing.T) {
	var buffer bytes.Buffer
	result, err := gzipCompress(object.CallArgs{
		Positional: object.Args{object.String("stream me")},
		Keyword:    map[string]object.Object{"dest": destRecorder(&buffer)},
	})
	if err != nil {
		t.Fatalf("compress() error = %v", err)
	}
	if _, ok := result.(object.Unit); !ok {
		t.Fatalf("compress(dest=...) = %v, want nil", result)
	}

	restored, err := gzipDecompress(object.CallArgs{Positional: object.Args{object.NewBytes(buffer.Bytes())}})
	if err != nil {
		t.Fatalf("decompress() error = %v", err)
	}
	if got := string(restored.(object.Bytes)); got != "stream me" {
		t.Fatalf("round trip = %q, want %q", got, "stream me")
	}
}

func TestLZWCompressToDest(t *testing.T) {
	var buffer bytes.Buffer
	result, err := lzwCompress(object.CallArgs{
		Positional: object.Args{object.String("stream me")},
		Keyword:    map[string]object.Object{"dest": destRecorder(&buffer)},
	})
	if err != nil {
		t.Fatalf("compress() error = %v", err)
	}
	if _, ok := result.(object.Unit); !ok {
		t.Fatalf("compress(dest=...) = %v, want nil", result)
	}

	restored, err := lzwDecompress(object.CallArgs{Positional: object.Args{object.NewBytes(buffer.Bytes())}})
	if err != nil {
		t.Fatalf("decompress() error = %v", err)
	}
	if got := string(restored.(object.Bytes)); got != "stream me" {
		t.Fatalf("round trip = %q, want %q", got, "stream me")
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
		Keyword:    map[string]object.Object{"dest": destRecorder(&buffer)},
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

func TestZipWriteAllToDest(t *testing.T) {
	files := object.NewDict()
	if err := files.Set(object.String("a.txt"), object.String("alpha")); err != nil {
		t.Fatal(err)
	}

	var buffer bytes.Buffer
	result, err := zipWriteAll(object.CallArgs{
		Positional: object.Args{files},
		Keyword:    map[string]object.Object{"dest": destRecorder(&buffer)},
	})
	if err != nil {
		t.Fatalf("write_all() error = %v", err)
	}
	if _, ok := result.(object.Unit); !ok {
		t.Fatalf("write_all(dest=...) = %v, want nil", result)
	}

	restored, err := zipReadAll(object.CallArgs{Positional: object.Args{object.NewBytes(buffer.Bytes())}})
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

func TestDestMustBeWriter(t *testing.T) {
	_, err := gzipCompress(object.CallArgs{
		Positional: object.Args{object.String("data")},
		Keyword:    map[string]object.Object{"dest": object.Integer(1)},
	})
	if err == nil {
		t.Fatal("compress(dest=1) succeeded, want TypeError")
	}
}
