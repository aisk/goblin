package http

import (
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/aisk/goblin/object"
)

type testReader struct {
	objectBase
	chunks    []object.Object
	readSizes []object.Integer
	closed    bool
}

func newTestReader(chunks ...object.Object) *testReader {
	return &testReader{objectBase: objectBase{typeName: "TestReader"}, chunks: chunks}
}

func (r *testReader) String() string            { return "<test_reader>" }
func (r *testReader) ToString() (string, error) { return r.String(), nil }
func (r *testReader) Attributes() []string      { return []string{"read", "close"} }
func (r *testReader) GetAttr(name string) (object.Object, error) {
	switch name {
	case "read":
		return &object.Function{Name: "read", Fn: func(args object.CallArgs) (object.Object, error) {
			ap := object.NewArgParser("read", args)
			size := ap.Int("size")
			if err := ap.Finish(); err != nil {
				return nil, err
			}
			r.readSizes = append(r.readSizes, size)
			if len(r.chunks) == 0 {
				return object.Bytes{}, nil
			}
			chunk := r.chunks[0]
			r.chunks = r.chunks[1:]
			return chunk, nil
		}}, nil
	case "close":
		return &object.Function{Name: "close", Fn: func(args object.CallArgs) (object.Object, error) {
			if err := object.RequireNoKeyword("close", args); err != nil {
				return nil, err
			}
			if len(args.Positional) != 0 {
				return nil, object.NewTypeError("close() takes no arguments")
			}
			r.closed = true
			return object.Nil, nil
		}}, nil
	default:
		return nil, object.NewAttributeError("TestReader has no attribute '%s'", name)
	}
}

var _ object.Object = (*testReader)(nil)

func moduleFunction(t *testing.T, name string) *object.Function {
	t.Helper()

	modObj, err := Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	mod, ok := modObj.(*object.Module)
	if !ok {
		t.Fatalf("Execute() returned %T", modObj)
	}

	member, ok := mod.Members[name]
	if !ok {
		t.Fatalf("http module missing %q", name)
	}

	fn, ok := member.(*object.Function)
	if !ok {
		t.Fatalf("http module member %q is %T", name, member)
	}

	return fn
}

// attr fetches an attribute from an object, failing the test on error.
func attr(t *testing.T, obj object.Object, name string) object.Object {
	t.Helper()
	v, err := obj.GetAttr(name)
	if err != nil {
		t.Fatalf("GetAttr(%q) error = %v", name, err)
	}
	return v
}

// callMethod fetches a method attribute and calls it with the given positional
// arguments.
func callMethod(t *testing.T, obj object.Object, name string, args ...object.Object) object.Object {
	t.Helper()
	fnObj := attr(t, obj, name)
	fn, ok := fnObj.(*object.Function)
	if !ok {
		t.Fatalf("%s is %T, want function", name, fnObj)
	}
	res, err := fn.Call(object.CallArgs{Positional: object.Args(args)})
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return res
}

// TestRequestAndDo exercises Request + header.set + Client.do, the path for a
// request that needs custom headers and a body.
func TestRequestAndDo(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != "POST" {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("X-Test"); got != "yes" {
			t.Fatalf("X-Test header = %q, want yes", got)
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		w.Header().Set("X-Reply", "ok")
		w.WriteHeader(201)
		_, _ = w.Write([]byte("received:" + string(data)))
	}))
	defer server.Close()

	// Request(method, url, body)
	reqObj, err := moduleFunction(t, "Request").Call(object.CallArgs{
		Positional: object.Args{
			object.String("POST"),
			object.String(server.URL),
			object.String("payload"),
		},
	})
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	req, ok := reqObj.(*Request)
	if !ok {
		t.Fatalf("Request() returned %T", reqObj)
	}

	// req.header.set("X-Test", "yes")
	header := attr(t, req, "header")
	callMethod(t, header, "set", object.String("X-Test"), object.String("yes"))

	// c = Client(); c.do(req)
	clientObj, err := moduleFunction(t, "Client").Call(object.CallArgs{})
	if err != nil {
		t.Fatalf("Client() error = %v", err)
	}
	if _, ok := clientObj.(*Client); !ok {
		t.Fatalf("Client() returned %T", clientObj)
	}
	respObj := callMethod(t, clientObj, "do", req)

	resp, ok := respObj.(*Response)
	if !ok {
		t.Fatalf("do() returned %T", respObj)
	}

	if code := attr(t, resp, "status_code").(object.Integer); code != 201 {
		t.Fatalf("status_code = %d, want 201", code)
	}
	if status := attr(t, resp, "status").String(); status != "201 Created" {
		t.Fatalf("status = %q, want \"201 Created\"", status)
	}

	// resp.header.get("X-Reply")
	respHeader := attr(t, resp, "header")
	reply := callMethod(t, respHeader, "get", object.String("X-Reply"))
	if got := reply.String(); got != "ok" {
		t.Fatalf("X-Reply = %q, want ok", got)
	}
}

func TestRequestBodyUsesDuckTypedReader(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			return
		}
		if got := string(data); got != "hello world" {
			t.Errorf("body = %q, want %q", got, "hello world")
		}
		w.WriteHeader(stdhttp.StatusNoContent)
	}))
	defer server.Close()

	reader := newTestReader(object.String("hello "), object.Bytes("world"), object.Nil)
	reqObj, err := moduleFunction(t, "Request").Call(object.CallArgs{Positional: object.Args{
		object.String("POST"), object.String(server.URL), reader,
	}})
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	req := reqObj.(*Request)
	if _, ok := attr(t, req, "body").(*Body); !ok {
		t.Fatalf("request body = %T, want *Body", attr(t, req, "body"))
	}

	clientObj, err := moduleFunction(t, "Client").Call(object.CallArgs{})
	if err != nil {
		t.Fatalf("Client() error = %v", err)
	}
	resp := callMethod(t, clientObj, "do", req).(*Response)
	defer resp.body.Close()

	if len(reader.readSizes) == 0 || reader.readSizes[0] <= 0 {
		t.Fatalf("read(size) calls = %v, want a positive requested size", reader.readSizes)
	}
	if !reader.closed {
		t.Fatal("duck reader close() was not called after sending request")
	}
}

func TestResponseBodyIsStreamingReader(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		_, _ = w.Write([]byte("abcdef"))
	}))
	defer server.Close()

	respObj, err := moduleFunction(t, "get").Call(object.CallArgs{
		Positional: object.Args{object.String(server.URL)},
	})
	if err != nil {
		t.Fatalf("get() error = %v", err)
	}
	body, ok := attr(t, respObj, "body").(*Body)
	if !ok {
		t.Fatalf("response body = %T, want *Body", attr(t, respObj, "body"))
	}

	first := callMethod(t, body, "read", object.Integer(3)).(object.Bytes)
	if got := string(first); got != "abc" {
		t.Fatalf("body.read(3) = %q, want abc", got)
	}
	rest := callMethod(t, body, "read").(object.Bytes)
	if got := string(rest); got != "def" {
		t.Fatalf("body.read() = %q, want def", got)
	}
	callMethod(t, body, "close")
	if closed := attr(t, body, "closed").(object.Bool); !closed.Bool() {
		t.Fatal("body.closed = false after close()")
	}
}

func TestGet(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != "GET" {
			t.Fatalf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name": "goblin", "stars": 42}`))
	}))
	defer server.Close()

	respObj, err := moduleFunction(t, "get").Call(object.CallArgs{
		Positional: object.Args{object.String(server.URL)},
	})
	if err != nil {
		t.Fatalf("get() error = %v", err)
	}
	resp := respObj.(*Response)

	// resp.json()
	result := callMethod(t, resp, "json")
	dict, ok := result.(*object.Dict)
	if !ok {
		t.Fatalf("json() returned %T, want dict", result)
	}
	nameVal, err := dict.Index(object.String("name"))
	if err != nil {
		t.Fatalf("dict[\"name\"] error = %v", err)
	}
	if got := nameVal.String(); got != "goblin" {
		t.Fatalf("name = %q, want goblin", got)
	}
	starsVal, err := dict.Index(object.String("stars"))
	if err != nil {
		t.Fatalf("dict[\"stars\"] error = %v", err)
	}
	if got := starsVal.(object.Integer); got != 42 {
		t.Fatalf("stars = %d, want 42", got)
	}
}

func TestPost(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != "POST" {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", ct)
		}
		data, _ := io.ReadAll(r.Body)
		if string(data) != `{"a":1}` {
			t.Fatalf("body = %q", string(data))
		}
		w.WriteHeader(204)
	}))
	defer server.Close()

	respObj, err := moduleFunction(t, "post").Call(object.CallArgs{
		Positional: object.Args{
			object.String(server.URL),
			object.String("application/json"),
			object.String(`{"a":1}`),
		},
	})
	if err != nil {
		t.Fatalf("post() error = %v", err)
	}
	resp := respObj.(*Response)
	if code := attr(t, resp, "status_code").(object.Integer); code != 204 {
		t.Fatalf("status_code = %d, want 204", code)
	}
}
