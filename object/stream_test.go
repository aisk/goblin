package object

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// duckObject builds a minimal duck-typed stream object out of a Module, whose
// GetAttr exposes the given members.
func duckObject(members map[string]Object) Object {
	return &Module{Name: "duck", Members: members}
}

func writerObject(t *testing.T, buffer *strings.Builder, result func(chunk Bytes) Object) Object {
	t.Helper()
	return duckObject(map[string]Object{
		"write": &Function{Name: "write", Fn: func(args CallArgs) (Object, error) {
			chunk, ok := args.Positional[0].(Bytes)
			if !ok {
				t.Fatalf("write() received %T, want Bytes", args.Positional[0])
			}
			value := result(chunk)
			if n, ok := value.(Integer); ok {
				if n >= 0 && int(n) <= len(chunk) {
					buffer.Write(chunk[:n])
				}
			} else {
				buffer.Write(chunk)
			}
			return value, nil
		}},
	})
}

func TestDuckWriterWritesWholeChunk(t *testing.T) {
	var buffer strings.Builder
	writer, err := NewDuckWriter("test", "dest", writerObject(t, &buffer, func(chunk Bytes) Object {
		return Integer(len(chunk))
	}))
	if err != nil {
		t.Fatalf("NewDuckWriter() error = %v", err)
	}
	n, err := writer.Write([]byte("hello"))
	if err != nil || n != 5 {
		t.Fatalf("Write() = %d, %v; want 5, nil", n, err)
	}
	if buffer.String() != "hello" {
		t.Fatalf("written = %q, want hello", buffer.String())
	}
}

func TestDuckWriterLoopsOnPartialWrites(t *testing.T) {
	var buffer strings.Builder
	writer, err := NewDuckWriter("test", "dest", writerObject(t, &buffer, func(chunk Bytes) Object {
		if len(chunk) > 2 {
			return Integer(2)
		}
		return Integer(len(chunk))
	}))
	if err != nil {
		t.Fatalf("NewDuckWriter() error = %v", err)
	}
	n, err := writer.Write([]byte("abcdefg"))
	if err != nil || n != 7 {
		t.Fatalf("Write() = %d, %v; want 7, nil", n, err)
	}
	if buffer.String() != "abcdefg" {
		t.Fatalf("written = %q, want abcdefg", buffer.String())
	}
}

func TestDuckWriterAcceptsNilReturn(t *testing.T) {
	var buffer strings.Builder
	writer, err := NewDuckWriter("test", "dest", writerObject(t, &buffer, func(Bytes) Object { return Nil }))
	if err != nil {
		t.Fatalf("NewDuckWriter() error = %v", err)
	}
	if n, err := writer.Write([]byte("data")); err != nil || n != 4 {
		t.Fatalf("Write() = %d, %v; want 4, nil", n, err)
	}
	if buffer.String() != "data" {
		t.Fatalf("written = %q, want data", buffer.String())
	}
}

func TestDuckWriterZeroProgressIsShortWrite(t *testing.T) {
	var buffer strings.Builder
	writer, err := NewDuckWriter("test", "dest", writerObject(t, &buffer, func(Bytes) Object { return Integer(0) }))
	if err != nil {
		t.Fatalf("NewDuckWriter() error = %v", err)
	}
	if _, err := writer.Write([]byte("data")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("Write() error = %v, want io.ErrShortWrite", err)
	}
}

func TestDuckWriterRejectsBadReturnAndCounts(t *testing.T) {
	var buffer strings.Builder
	writer, err := NewDuckWriter("test", "dest", writerObject(t, &buffer, func(Bytes) Object { return String("5") }))
	if err != nil {
		t.Fatalf("NewDuckWriter() error = %v", err)
	}
	if _, err := writer.Write([]byte("data")); err == nil || !strings.Contains(err.Error(), "must return int or nil") {
		t.Fatalf("Write() error = %v, want return-type TypeError", err)
	}

	writer, err = NewDuckWriter("test", "dest", writerObject(t, &buffer, func(chunk Bytes) Object {
		return Integer(len(chunk) + 1)
	}))
	if err != nil {
		t.Fatalf("NewDuckWriter() error = %v", err)
	}
	if _, err := writer.Write([]byte("data")); err == nil || !strings.Contains(err.Error(), "returned") {
		t.Fatalf("Write() error = %v, want invalid-count ValueError", err)
	}
}

func TestDuckWriterRequiresWriteMethod(t *testing.T) {
	if _, err := NewDuckWriter("test", "dest", Integer(1)); err == nil || !strings.Contains(err.Error(), "write(data)") {
		t.Fatalf("NewDuckWriter() error = %v, want missing write(data) TypeError", err)
	}
	notCallable := duckObject(map[string]Object{"write": String("nope")})
	if _, err := NewDuckWriter("test", "dest", notCallable); err == nil || !strings.Contains(err.Error(), "must be callable") {
		t.Fatalf("NewDuckWriter() error = %v, want not-callable TypeError", err)
	}
}

func TestDuckWriterOptionalClose(t *testing.T) {
	var buffer strings.Builder
	closed := false
	value := duckObject(map[string]Object{
		"write": &Function{Name: "write", Fn: func(args CallArgs) (Object, error) {
			buffer.Write([]byte(args.Positional[0].(Bytes)))
			return Nil, nil
		}},
		"close": &Function{Name: "close", Fn: func(CallArgs) (Object, error) {
			closed = true
			return Nil, nil
		}},
	})
	writer, err := NewDuckWriter("test", "dest", value)
	if err != nil {
		t.Fatalf("NewDuckWriter() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !closed {
		t.Fatal("close() was not invoked")
	}
	if _, err := writer.Write([]byte("x")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Write() after Close() error = %v, want io.ErrClosedPipe", err)
	}
}

func TestDuckReaderReadsChunksAndCloses(t *testing.T) {
	chunks := []Object{String("hello "), Bytes("world"), Nil}
	closed := false
	value := duckObject(map[string]Object{
		"read": &Function{Name: "read", Fn: func(args CallArgs) (Object, error) {
			if len(chunks) == 0 {
				return Bytes{}, nil
			}
			chunk := chunks[0]
			chunks = chunks[1:]
			return chunk, nil
		}},
		"close": &Function{Name: "close", Fn: func(CallArgs) (Object, error) {
			closed = true
			return Nil, nil
		}},
	})
	reader, err := NewDuckReader("test", "src", value)
	if err != nil {
		t.Fatalf("NewDuckReader() error = %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("ReadAll() = %q, want %q", data, "hello world")
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !closed {
		t.Fatal("close() was not invoked")
	}
}

func TestDuckReaderRequiresReadMethod(t *testing.T) {
	if _, err := NewDuckReader("test", "src", Integer(1)); err == nil || !strings.Contains(err.Error(), "read(size)") {
		t.Fatalf("NewDuckReader() error = %v, want missing read(size) TypeError", err)
	}
}
