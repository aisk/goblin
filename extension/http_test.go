package extension

import (
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/aisk/goblin/object"
)

func httpFunction(t *testing.T, name string) *object.Function {
	t.Helper()

	modObj, err := ExecuteHttp()
	if err != nil {
		t.Fatalf("ExecuteHttp() error = %v", err)
	}

	mod, ok := modObj.(*object.Module)
	if !ok {
		t.Fatalf("ExecuteHttp() returned %T", modObj)
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

func TestHTTPRequest(t *testing.T) {
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

	headers := object.NewDict()
	headers.Set(object.String("X-Test"), object.String("yes"))

	respObj, err := httpFunction(t, "request").Call(object.CallArgs{
		Positional: object.Args{
			object.String("POST"),
			object.String(server.URL),
			object.String("payload"),
			headers,
		},
	})
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}

	resp, ok := respObj.(*HTTPResponse)
	if !ok {
		t.Fatalf("request() returned %T", respObj)
	}
	if resp.Status != 201 {
		t.Fatalf("status = %d, want 201", resp.Status)
	}

	reply, err := resp.Headers.Index(object.String("X-Reply"))
	if err != nil {
		t.Fatalf("headers[\"X-Reply\"] error = %v", err)
	}
	if got := reply.String(); got != "ok" {
		t.Fatalf("X-Reply = %q, want ok", got)
	}

	readFnObj, err := resp.Body.GetAttr("read")
	if err != nil {
		t.Fatalf("body.read attr error = %v", err)
	}
	readFn, ok := readFnObj.(*object.Function)
	if !ok {
		t.Fatalf("body.read is %T", readFnObj)
	}
	bodyObj, err := readFn.Call(object.CallArgs{})
	if err != nil {
		t.Fatalf("body.read() error = %v", err)
	}
	if got := bodyObj.String(); got != "received:payload" {
		t.Fatalf("body.read() = %q, want received:payload", got)
	}
}

func TestHTTPRequestDefaultsToGetWithURL(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != "GET" {
			t.Fatalf("method = %q, want GET", r.Method)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	respObj, err := httpFunction(t, "request").Call(object.CallArgs{
		Positional: object.Args{object.String(server.URL)},
	})
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}
	resp := respObj.(*HTTPResponse)
	bodyObj, err := resp.Body.Read(object.CallArgs{})
	if err != nil {
		t.Fatalf("body.read() error = %v", err)
	}
	if got := bodyObj.String(); got != "ok" {
		t.Fatalf("body.read() = %q, want ok", got)
	}
}
