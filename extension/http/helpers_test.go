package http

import (
	"errors"
	"fmt"
	"io"
	"net"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/object"
)

func kw(name string, v object.Object) object.KwArg { return object.KwArg{Name: name, Value: v} }

func str(s string) object.Object { return object.String(s) }

func args(items ...object.Object) []object.Object { return items }

func dict(pairs ...object.Object) *object.Dict {
	d := object.NewDict()
	for i := 0; i < len(pairs); i += 2 {
		d.Set(pairs[i], pairs[i+1])
	}
	return d
}

func list(items ...object.Object) *object.List { return &object.List{Elements: items} }

func member(t *testing.T, name string) *object.Function {
	t.Helper()
	m, err := Execute()
	if err != nil {
		t.Fatal(err)
	}
	f, ok := m.(*object.Module).Members[name].(*object.Function)
	if !ok {
		t.Fatalf("http has no function %q", name)
	}
	return f
}

func callErr(t *testing.T, f *object.Function, positional []object.Object, kws ...object.KwArg) (object.Object, error) {
	t.Helper()
	return f.Call(object.CallArgs{Positional: positional, Keyword: kws})
}

func call(t *testing.T, f *object.Function, positional []object.Object, kws ...object.KwArg) object.Object {
	t.Helper()
	v, err := callErr(t, f, positional, kws...)
	if err != nil {
		t.Fatalf("%s() error = %v", f.Name, err)
	}
	return v
}

func get(t *testing.T, v object.Object, name string) object.Object {
	t.Helper()
	got, err := v.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func method(t *testing.T, v object.Object, name string, positional []object.Object, kws ...object.KwArg) object.Object {
	t.Helper()
	return call(t, get(t, v, name).(*object.Function), positional, kws...)
}

func methodErr(t *testing.T, v object.Object, name string, positional []object.Object, kws ...object.KwArg) (object.Object, error) {
	t.Helper()
	return callErr(t, get(t, v, name).(*object.Function), positional, kws...)
}

// text is response.text().
func text(t *testing.T, msg object.Object) string {
	t.Helper()
	return string(method(t, msg, "text", nil).(object.String))
}

func body(t *testing.T, msg object.Object) *filestream.File {
	t.Helper()
	return get(t, msg, "body").(*filestream.File)
}

func newClientT(t *testing.T, kws ...object.KwArg) object.Object {
	t.Helper()
	return call(t, clientType.Function, nil, kws...)
}

func newRequestT(t *testing.T, method, url string, kws ...object.KwArg) object.Object {
	t.Helper()
	return call(t, requestType.Function, args(str(method), str(url)), kws...)
}

func field(t *testing.T, err error, name string) object.Object {
	t.Helper()
	var e *object.Error
	if !errors.As(err, &e) {
		t.Fatalf("%v is not a Goblin error", err)
	}
	v, ok := e.Field(name)
	if !ok {
		t.Fatalf("error %q has no data %q", err, name)
	}
	return v
}

// wantErr checks an error's kind and its message, the text `e.message`
// reads (without the cause a wrapped error appends).
func wantErr(t *testing.T, err error, kind *object.Error, msg string) {
	t.Helper()
	var e *object.Error
	if err == nil || !errors.Is(err, kind) || !errors.As(err, &e) || e.Value != msg {
		t.Fatalf("error = %v, want %s %q", err, kind.Value, msg)
	}
}

func wantErrPrefix(t *testing.T, err error, kind *object.Error, prefix string) {
	t.Helper()
	if err == nil || !errors.Is(err, kind) || !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("error = %v, want %s starting %q", err, kind.Value, prefix)
	}
}

// seen is what a recording handler saw of one request.
type seen struct {
	method        string
	path          string
	query         string
	host          string
	header        stdhttp.Header
	body          string
	contentLength int64
	chunked       bool
	proto         string
}

// recorder records every request it serves, then calls next (or answers
// "ok").
type recorder struct {
	mu   sync.Mutex
	reqs []seen
	next stdhttp.HandlerFunc
}

func (rec *recorder) ServeHTTP(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	data, _ := io.ReadAll(r.Body)
	rec.mu.Lock()
	rec.reqs = append(rec.reqs, seen{
		method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, host: r.Host,
		header: r.Header.Clone(), body: string(data), contentLength: r.ContentLength,
		chunked: len(r.TransferEncoding) > 0 && r.TransferEncoding[0] == "chunked", proto: r.Proto,
	})
	rec.mu.Unlock()
	if rec.next != nil {
		rec.next(w, r)
		return
	}
	io.WriteString(w, "ok")
}

func (rec *recorder) last(t *testing.T) seen {
	t.Helper()
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.reqs) == 0 {
		t.Fatal("the server saw no request")
	}
	return rec.reqs[len(rec.reqs)-1]
}

func (rec *recorder) all() []seen {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return append([]seen(nil), rec.reqs...)
}

// names lists the header names a server received, sorted.
func names(h stdhttp.Header) []string {
	var out []string
	for name := range h {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func sameNames(t *testing.T, what string, h stdhttp.Header, want ...string) {
	t.Helper()
	sort.Strings(want)
	if got := names(h); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("%s: the server received headers %v (%v), want exactly %v", what, got, h, want)
	}
}

// connCounter counts the connections a test server accepts.
type connCounter struct{ n atomic.Int32 }

func (c *connCounter) hook(_ net.Conn, state stdhttp.ConnState) {
	if state == stdhttp.StateNew {
		c.n.Add(1)
	}
}

func countedServer(h stdhttp.Handler) (*httptest.Server, *connCounter) {
	srv := httptest.NewUnstartedServer(h)
	cc := &connCounter{}
	srv.Config.ConnState = cc.hook
	srv.Start()
	return srv, cc
}

func goblinFile(t *testing.T, path string) *filestream.File {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return filestream.Open(path, file, false)
}

// readable is a duck-typed readable over r, with a close() that records
// that it was called.
func readable(r io.Reader, closed *bool) object.Object {
	return &object.Module{Name: "readable", Members: map[string]object.Object{
		"read": &object.Function{Name: "read", Fn: func(a object.CallArgs) (object.Object, error) {
			n := 4096
			if len(a.Positional) > 0 {
				n = int(a.Positional[0].(object.Integer))
			}
			buf := make([]byte, n)
			k, err := r.Read(buf)
			if err != nil && err != io.EOF {
				return nil, err
			}
			return object.Bytes(buf[:k]), nil
		}},
		"close": &object.Function{Name: "close", Fn: func(object.CallArgs) (object.Object, error) {
			*closed = true
			return object.Nil, nil
		}},
	}}
}

func sortStrings(s []string) []string {
	sort.Strings(s)
	return s
}
