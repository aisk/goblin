package object

import (
	"errors"
	"io"
	"strings"
	"testing"
)

type pathStub struct {
	OpaqueBase
	raw string
}

func (p *pathStub) String() string                 { return p.raw }
func (p *pathStub) ToString() (string, error)      { return p.raw, nil }
func (p *pathStub) GetAttr(string) (Object, error) { return nil, NewAttributeError("no") }
func (p *pathStub) Attributes() []string           { return nil }
func (p *pathStub) FilePath() string               { return p.raw }

func TestArgParserPath(t *testing.T) {
	for _, arg := range []Object{String("a/b"), &pathStub{OpaqueBase: MakeOpaqueBase("Path"), raw: "a/b"}} {
		p := NewArgParser("read", CallArgs{Positional: Args{arg}})
		got := p.Path("path")
		if err := p.Finish(); err != nil || got != "a/b" {
			t.Fatalf("Path(%v) = %q, %v", arg, got, err)
		}
	}
	p := NewArgParser("read", CallArgs{Positional: Args{Integer(1)}})
	p.Path("path")
	err := p.Finish()
	if err == nil || err.Error() != "read() argument 'path' must be String or Path, got Int" || !errors.Is(err, TypeError) {
		t.Fatalf("Path(1) error = %v", err)
	}

	p = NewArgParser("run", CallArgs{Keyword: Kwargs{{Name: "cwd", Value: Nil}}})
	if _, ok := p.NullablePath("cwd"); ok || p.Finish() != nil {
		t.Fatal("NullablePath(nil) reported a path")
	}
}

func TestArgParserData(t *testing.T) {
	reader := &Module{Name: "reader", Members: map[string]Object{}}
	chunks := []Object{Bytes("ab"), String("c"), Bytes("")}
	reader.Members["read"] = &Function{Name: "read", Fn: func(CallArgs) (Object, error) {
		v := chunks[0]
		chunks = chunks[1:]
		return v, nil
	}}
	closed := false
	reader.Members["close"] = &Function{Name: "close", Fn: func(CallArgs) (Object, error) {
		closed = true
		return Nil, nil
	}}
	for _, arg := range []Object{String("abc"), Bytes("abc"), reader} {
		p := NewArgParser("decode", CallArgs{Positional: Args{arg}})
		r := p.Data("data")
		if err := p.Finish(); err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(r)
		if err != nil || string(got) != "abc" {
			t.Fatalf("Data(%v) read %q, %v", arg, got, err)
		}
		if c, ok := r.(io.Closer); ok {
			c.Close()
		}
	}
	if closed {
		t.Fatal("Data closed the stream it was handed")
	}

	p := NewArgParser("decode", CallArgs{Positional: Args{Integer(1)}})
	p.Data("data")
	if err := p.Finish(); err == nil || err.Error() != "decode() argument 'data' must be String, Bytes or have a read(size) method, got Int" {
		t.Fatalf("Data(1) error = %v", err)
	}
}

func TestArgParserDataBytes(t *testing.T) {
	p := NewArgParser("decode", CallArgs{Positional: Args{String("xyz")}})
	if got := p.DataBytes("data"); p.Finish() != nil || string(got) != "xyz" {
		t.Fatalf("DataBytes(String) = %q", got)
	}
	failing := &Module{Name: "reader", Members: map[string]Object{
		"read": &Function{Name: "read", Fn: func(CallArgs) (Object, error) {
			return nil, NewError("boom")
		}},
	}}
	p = NewArgParser("decode", CallArgs{Positional: Args{failing}})
	p.DataBytes("data")
	if err := p.Finish(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("DataBytes(failing reader) error = %v", err)
	}
}

func TestOpaqueConstructorRaisesWithHint(t *testing.T) {
	ctor := NewOpaqueConstructor("File", "fs.open(path)")
	_, err := ctor.Function.Call(CallArgs{})
	if err == nil || err.Error() != "File cannot be constructed; use fs.open(path)" || !errors.Is(err, TypeError) {
		t.Fatalf("File() error = %v", err)
	}
}
