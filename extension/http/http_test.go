package http

import (
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/aisk/goblin/object"
)

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
