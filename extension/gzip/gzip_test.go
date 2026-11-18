package gzip

import (
	"bytes"
	gogzip "compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

func fn(t *testing.T, name string) *object.Function {
	t.Helper()
	module, err := Execute()
	if err != nil {
		t.Fatal(err)
	}
	return module.(*object.Module).Members[name].(*object.Function)
}

func goGzip(t *testing.T, data string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gogzip.NewWriter(&buf)
	_, _ = w.Write([]byte(data))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRoundTripAndInterop(t *testing.T) {
	for _, level := range []object.Object{object.Nil, object.Integer(1), object.Integer(9)} {
		call := object.CallArgs{Positional: object.Args{object.String("hello, Goblin")}, Keyword: object.Kwargs{{Name: "level", Value: level}}}
		packed := modtest.CallModule(t, Execute, "compress", call).(object.Bytes)
		// Go's own reader (the format gzip(1) writes) reads it back.
		r, err := gogzip.NewReader(bytes.NewReader(packed))
		if err != nil {
			t.Fatal(err)
		}
		plain, _ := io.ReadAll(r)
		if string(plain) != "hello, Goblin" {
			t.Fatalf("level %v: decompressed %q", level, plain)
		}
		back := modtest.CallModule(t, Execute, "decompress", object.CallArgs{Positional: object.Args{packed}})
		if !modtest.Equal(back, object.NewBytes([]byte("hello, Goblin"))) {
			t.Fatalf("decompress = %v", back)
		}
	}
	// Data written by another implementation, several members in a row.
	data := append(goGzip(t, "one "), goGzip(t, "two")...)
	back := modtest.CallModule(t, Execute, "decompress", object.CallArgs{Positional: object.Args{object.NewBytes(data)}})
	if !modtest.Equal(back, object.NewBytes([]byte("one two"))) {
		t.Fatalf("multi-member decompress = %v", back)
	}
	empty := modtest.CallModule(t, Execute, "compress", object.CallArgs{Positional: object.Args{object.NewBytes(nil)}})
	if back := modtest.CallModule(t, Execute, "decompress", object.CallArgs{Positional: object.Args{empty}}); back.(object.Bytes).Size() != object.Integer(0) {
		t.Fatalf("empty round trip = %v", back)
	}
}

func TestErrors(t *testing.T) {
	packed := goGzip(t, "hello, Goblin")
	cases := []struct {
		fn   string
		args object.CallArgs
		kind *object.Error
		msg  string
	}{
		{"compress", object.CallArgs{Positional: object.Args{object.String("x"), object.Integer(0)}}, object.ValueError, "compress() argument 'level' must be between 1 and 9, got 0"},
		{"compress", object.CallArgs{Positional: object.Args{object.String("x"), object.Integer(10)}}, object.ValueError, "compress() argument 'level' must be between 1 and 9, got 10"},
		{"compress", object.CallArgs{Positional: object.Args{object.String("x"), object.String("9")}}, object.TypeError, "compress() argument 'level' must be Int or Nil, got String"},
		{"compress", object.CallArgs{Positional: object.Args{object.Integer(1)}}, object.TypeError, "compress() argument 'data' must be String, Bytes or have a read(size) method, got Int"},
		{"decompress", object.CallArgs{Positional: object.Args{object.NewBytes(nil)}}, object.ParseError, "decompress() not gzip data"},
		{"decompress", object.CallArgs{Positional: object.Args{object.NewBytes([]byte("plain text"))}}, object.ParseError, "decompress() not gzip data"},
		{"decompress", object.CallArgs{Positional: object.Args{object.NewBytes([]byte("hello"))}}, object.ParseError, "decompress() not gzip data"},
		{"decompress", object.CallArgs{Positional: object.Args{object.NewBytes(packed[:len(packed)-4])}}, object.ParseError, "decompress() corrupt gzip data"},
		{"decompress", object.CallArgs{Positional: object.Args{object.String("text")}}, object.TypeError, "decompress() argument 'data' must be Bytes or have a read(size) method, got String"},
	}
	for _, c := range cases {
		_, err := fn(t, c.fn).Call(c.args)
		if err == nil || !errors.Is(err, c.kind) || err.Error() != c.msg {
			t.Errorf("%s(%v): err = %v, want %q", c.fn, c.args.Positional, err, c.msg)
		}
	}
}

func call(t *testing.T, name string, positional []object.Object, kws ...object.KwArg) (object.Object, error) {
	t.Helper()
	return fn(t, name).Call(object.CallArgs{Positional: positional, Keyword: kws})
}

func mustCall(t *testing.T, name string, positional []object.Object, kws ...object.KwArg) object.Object {
	t.Helper()
	v, err := call(t, name, positional, kws...)
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return v
}

func callMethod(t *testing.T, v object.Object, name string, positional ...object.Object) (object.Object, error) {
	t.Helper()
	m, err := v.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	return m.(*object.Function).Call(object.CallArgs{Positional: positional})
}

func mustMethod(t *testing.T, v object.Object, name string, positional ...object.Object) object.Object {
	t.Helper()
	got, err := callMethod(t, v, name, positional...)
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return got
}

func strs(t *testing.T, items []object.Object) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = string(item.(object.String))
	}
	return out
}

func pathOf(t *testing.T, err error) string {
	t.Helper()
	var e *object.Error
	if !errors.As(err, &e) {
		t.Fatalf("%v is not a Goblin error", err)
	}
	p, ok := e.Field("path")
	if !ok {
		t.Fatalf("error %q carries no path", err)
	}
	return p.(*pathvalue.Path).FilePath()
}

func TestOpenRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.csv.gz")
	w := mustCall(t, "open", []object.Object{object.String(path)}, object.KwArg{Name: "write", Value: object.True}, object.KwArg{Name: "level", Value: object.Integer(9)})
	if w.(*filestream.File).String() != "<File "+path+">" {
		t.Fatalf("shows as %v", w)
	}
	if n := mustMethod(t, w, "write", object.String("a,b\n")); n != object.Integer(4) {
		t.Fatalf("write returned %v, want the bytes given", n)
	}
	mustMethod(t, w, "write", object.NewBytes([]byte("1,2\nlast")))
	if _, err := callMethod(t, w, "read"); !errors.Is(err, object.ValueError) {
		t.Fatalf("read on a writer = %v", err)
	}
	mustMethod(t, w, "close")
	mustMethod(t, w, "close")

	// Go's reader (the format gzip(1) writes) reads it back.
	raw, _ := os.ReadFile(path)
	zr, err := gogzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := io.ReadAll(zr)
	if string(plain) != "a,b\n1,2\nlast" {
		t.Fatalf("written %q", plain)
	}

	r := mustCall(t, "open", []object.Object{pathvalue.NewPath(path)})
	if line := mustMethod(t, r, "read_line"); line != object.String("a,b") {
		t.Fatalf("read_line = %v", line)
	}
	lines, err := object.Collect(r)
	if err != nil || !reflect.DeepEqual(strs(t, lines), []string{"1,2", "last"}) {
		t.Fatalf("lines = %v, %v", lines, err)
	}
	if _, err := callMethod(t, r, "write", object.String("x")); !errors.Is(err, object.ValueError) {
		t.Fatalf("write on a reader = %v", err)
	}
	mustMethod(t, r, "close")

	r = mustCall(t, "open", []object.Object{object.String(path)})
	if all := mustMethod(t, r, "read"); string(all.(object.Bytes)) != "a,b\n1,2\nlast" {
		t.Fatalf("read() = %v", all)
	}
	mustMethod(t, r, "close")
}

type trackedFile struct {
	io.ReadCloser
	closed *int
}

func (f trackedFile) Close() error {
	*f.closed++
	return f.ReadCloser.Close()
}

func TestReadLines(t *testing.T) {
	closed := 0
	saved := openFile
	openFile = func(path string) (io.ReadCloser, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return trackedFile{f, &closed}, nil
	}
	defer func() { openFile = saved }()

	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl.gz")
	os.WriteFile(path, goGzip(t, "one\r\ntwo\n\nthree"), 0o644)
	lines := mustCall(t, "read_lines", []object.Object{object.String(path)})
	if lines.(*filestream.Lines).Constructor() != filestream.LinesType.Function {
		t.Fatal("read_lines does not return an fs.Lines")
	}
	items, err := object.Collect(lines)
	if err != nil || !reflect.DeepEqual(strs(t, items), []string{"one", "two", "", "three"}) {
		t.Fatalf("lines = %v, %v", items, err)
	}
	if closed != 1 {
		t.Fatalf("closed %d times after a full pass", closed)
	}
	it, err := object.Iterate(lines)
	if err != nil {
		t.Fatal(err)
	}
	it.Next()
	it.Close()
	if closed != 2 {
		t.Fatalf("closed %d times after break", closed)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove after break: %v", err)
	}

	// Not gzip: raised when the loop starts, carrying the path.
	plain := filepath.Join(dir, "plain.gz")
	os.WriteFile(plain, []byte("plain text\n"), 0o644)
	_, err = object.Iterate(mustCall(t, "read_lines", []object.Object{object.String(plain)}))
	if !errors.Is(err, object.ParseError) || err.Error() != `read_lines() "`+plain+`" is not gzip data` || pathOf(t, err) != plain {
		t.Fatalf("not gzip = %v", err)
	}
	if closed != 3 {
		t.Fatalf("closed %d times after a header error", closed)
	}
	_, err = object.Iterate(mustCall(t, "read_lines", []object.Object{object.String(filepath.Join(dir, "missing.gz"))}))
	if !errors.Is(err, object.NotExistError) || pathOf(t, err) != filepath.Join(dir, "missing.gz") {
		t.Fatalf("missing = %v", err)
	}
	// The call is named once, as fs.read_lines names it.
	if want := `read_lines() cannot open "` + filepath.Join(dir, "missing.gz") + `": no such file or directory`; err.Error() != want {
		t.Fatalf("missing = %q, want %q", err, want)
	}
	_, err = object.Iterate(mustCall(t, "read_lines", []object.Object{object.String(dir)}))
	if want := `read_lines() cannot read "` + dir + `": is a directory`; err == nil || err.Error() != want || errors.Is(err, object.NetworkError) {
		t.Fatalf("directory = %v, want %q", err, want)
	}

	// Truncated: the lines before the damage arrive, then the read that
	// reaches it raises.
	truncated := filepath.Join(dir, "truncated.gz")
	whole := goGzip(t, strings.Repeat("line\n", 1000))
	os.WriteFile(truncated, whole[:len(whole)-6], 0o644)
	_, err = object.Collect(mustCall(t, "read_lines", []object.Object{object.String(truncated)}))
	if !errors.Is(err, object.ParseError) || err.Error() != `read_lines() corrupt gzip data in "`+truncated+`"` || pathOf(t, err) != truncated {
		t.Fatalf("truncated = %v", err)
	}
	f := mustCall(t, "open", []object.Object{object.String(truncated)})
	_, err = callMethod(t, f, "read")
	if !errors.Is(err, object.ParseError) || err.Error() != `read() corrupt gzip data in "`+truncated+`"` {
		t.Fatalf("truncated read() = %v", err)
	}
	mustMethod(t, f, "close")

	bad := filepath.Join(dir, "bad.gz")
	os.WriteFile(bad, goGzip(t, "ok\nok\nbad\xff\n"), 0o644)
	_, err = object.Collect(mustCall(t, "read_lines", []object.Object{object.String(bad)}))
	var e *object.Error
	errors.As(err, &e)
	if line, _ := e.Field("line"); !errors.Is(err, object.ValueError) || line != object.Integer(3) || pathOf(t, err) != bad {
		t.Fatalf("invalid UTF-8 = %v", err)
	}
}

func TestOpenErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.gz")
	if _, err := call(t, "open", []object.Object{object.String(path)}, object.KwArg{Name: "level", Value: object.Integer(5)}); !errors.Is(err, object.ValueError) {
		t.Fatalf("level without write = %v", err)
	}
	if _, err := call(t, "open", []object.Object{object.String(path)}, object.KwArg{Name: "write", Value: object.True}, object.KwArg{Name: "level", Value: object.Integer(0)}); err == nil || err.Error() != "open() argument 'level' must be between 1 and 9, got 0" {
		t.Fatalf("level 0 = %v", err)
	}
	if _, err := call(t, "open", []object.Object{object.String(path)}); !errors.Is(err, object.NotExistError) || pathOf(t, err) != path {
		t.Fatalf("missing = %v", err)
	}
	os.WriteFile(path, []byte("text"), 0o644)
	if _, err := call(t, "open", []object.Object{object.String(path)}); !errors.Is(err, object.ParseError) || err.Error() != `open() "`+path+`" is not gzip data` {
		t.Fatalf("not gzip = %v", err)
	}

	// A writer that is never closed leaves an incomplete file.
	unfinished := filepath.Join(dir, "unfinished.gz")
	w := mustCall(t, "open", []object.Object{object.String(unfinished)}, object.KwArg{Name: "write", Value: object.True})
	mustMethod(t, w, "write", object.String(strings.Repeat("data ", 10000)))
	raw, _ := os.ReadFile(unfinished)
	if _, err := call(t, "decompress", []object.Object{object.NewBytes(raw)}); !errors.Is(err, object.ParseError) {
		t.Fatalf("decompress of an unfinished file = %v", err)
	}
	mustMethod(t, w, "close")
}
