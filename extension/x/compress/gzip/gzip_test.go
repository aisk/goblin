package gzip

import (
	"bytes"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestGzipRoundTrip(t *testing.T) {
	compressed, err := gzipCompress(object.CallArgs{Positional: object.Args{object.String("goblin goblin goblin")}})
	if err != nil {
		t.Fatal(err)
	}
	decompressed, err := gzipDecompress(object.CallArgs{Positional: object.Args{compressed}})
	if err != nil {
		t.Fatal(err)
	}
	if string(decompressed.(object.Bytes)) != "goblin goblin goblin" {
		t.Fatalf("decompressed = %v", decompressed)
	}
}

func TestGzipCompressToDest(t *testing.T) {
	var buffer bytes.Buffer
	result, err := gzipCompress(object.CallArgs{
		Positional: object.Args{object.String("stream me")},
		Keyword:    object.Kwargs{{Name: "dest", Value: modtest.DestRecorder(&buffer)}},
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

func TestDestMustBeWriter(t *testing.T) {
	_, err := gzipCompress(object.CallArgs{
		Positional: object.Args{object.String("data")},
		Keyword:    object.Kwargs{{Name: "dest", Value: object.Integer(1)}},
	})
	if err == nil {
		t.Fatal("compress(dest=1) succeeded, want TypeError")
	}
}
