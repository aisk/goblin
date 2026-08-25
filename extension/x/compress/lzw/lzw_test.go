package lzw

import (
	"bytes"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestLZWCompressToDest(t *testing.T) {
	var buffer bytes.Buffer
	result, err := lzwCompress(object.CallArgs{
		Positional: object.Args{object.String("stream me")},
		Keyword:    map[string]object.Object{"dest": modtest.DestRecorder(&buffer)},
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
