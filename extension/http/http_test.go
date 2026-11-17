package http

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net"
	stdhttp "net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/extension/internal/urlvalue"
	"github.com/aisk/goblin/object"
)

func kw(name string, v object.Object) object.KwArg { return object.KwArg{Name: name, Value: v} }

func str(s string) object.Object { return object.String(s) }

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

func text(t *testing.T, resp object.Object) string {
	t.Helper()
	return string(method(t, resp, "text", nil).(object.String))
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

// echo answers with what it received, as text lines a test can check.
func echo(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	body, _ := io.ReadAll(r.Body)
	fmt.Fprintf(w, "method=%s\n", r.Method)
	fmt.Fprintf(w, "query=%s\n", r.URL.RawQuery)
	fmt.Fprintf(w, "content-type=%s\n", r.Header.Get("Content-Type"))
	fmt.Fprintf(w, "content-length=%d\n", r.ContentLength)
	fmt.Fprintf(w, "transfer-encoding=%s\n", strings.Join(r.TransferEncoding, ","))
	fmt.Fprintf(w, "user-agent=%s\n", r.Header.Get("User-Agent"))
	fmt.Fprintf(w, "x-token=%s\n", strings.Join(r.Header.Values("X-Token"), "|"))
	fmt.Fprintf(w, "host=%s\n", r.Host)
	fmt.Fprintf(w, "cookie=%s\n", r.Header.Get("Cookie"))
	fmt.Fprintf(w, "body=%s\n", body)
}

func lineOf(body, name string) string {
	for _, line := range strings.Split(body, "\n") {
		if v, ok := strings.CutPrefix(line, name+"="); ok {
			return v
		}
	}
	return "<missing " + name + ">"
}

func TestRequestFunctionsAndKeywords(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(echo))
	defer srv.Close()

	for _, c := range []struct{ fn, method string }{{"get", "GET"}, {"delete", "DELETE"}, {"post", "POST"}, {"put", "PUT"}, {"patch", "PATCH"}} {
		resp := call(t, member(t, c.fn), []object.Object{str(srv.URL + "/x?a=1")}, kw("params", dict(str("q"), str("a b"), str("tag"), list(str("x"), str("y")), str("skip"), object.Nil)))
		body := text(t, resp)
		if lineOf(body, "method") != c.method || lineOf(body, "query") != "a=1&q=a+b&tag=x&tag=y" || lineOf(body, "user-agent") != "goblin" {
			t.Errorf("%s: %s", c.fn, body)
		}
		if get(t, resp, "status") != object.Integer(200) || get(t, resp, "status_text") != str("OK") || get(t, resp, "success") != object.True {
			t.Errorf("%s status", c.fn)
		}
	}
	resp := call(t, member(t, "request"), []object.Object{str("OPTIONS"), urlvalue.New(mustURL(t, srv.URL))})
	if lineOf(text(t, resp), "method") != "OPTIONS" {
		t.Fatal("request(OPTIONS)")
	}
	head := call(t, member(t, "request"), []object.Object{str("HEAD"), str(srv.URL)})
	if text(t, head) != "" {
		t.Fatal("HEAD has a body")
	}

	post := member(t, "post")
	body := text(t, call(t, post, []object.Object{str(srv.URL)}, kw("json", dict(str("name"), str("Ada")))))
	if lineOf(body, "content-type") != "application/json" || lineOf(body, "body") != `{"name":"Ada"}` || lineOf(body, "content-length") != "14" {
		t.Errorf("json=: %s", body)
	}
	body = text(t, call(t, post, []object.Object{str(srv.URL)}, kw("form", dict(str("email"), str("a@b.org"), str("n"), object.Integer(2)))))
	if lineOf(body, "content-type") != "application/x-www-form-urlencoded" || lineOf(body, "body") != "email=a%40b.org&n=2" {
		t.Errorf("form=: %s", body)
	}
	body = text(t, call(t, post, []object.Object{str(srv.URL)}, kw("body", object.Bytes("raw")), kw("headers", dict(str("content-type"), str("text/plain"), str("User-Agent"), str("mine/1"), str("X-Token"), list(str("a"), str("b")), str("Host"), str("example.test")))))
	if lineOf(body, "content-type") != "text/plain" || lineOf(body, "body") != "raw" || lineOf(body, "user-agent") != "mine/1" || lineOf(body, "x-token") != "a|b" || lineOf(body, "host") != "example.test" {
		t.Errorf("body= and headers=: %s", body)
	}
	// A header you pass wins over the library's.
	body = text(t, call(t, post, []object.Object{str(srv.URL)}, kw("json", list()), kw("headers", dict(str("CONTENT-TYPE"), str("application/vnd+json")))))
	if lineOf(body, "content-type") != "application/vnd+json" {
		t.Errorf("Content-Type override: %s", body)
	}

	// A request held as a Dict, expanded at the call.
	resp = call(t, post, nil, kw("url", str(srv.URL)), kw("body", str("dict")), kw("timeout", object.Integer(5)), kw("check", object.False))
	if lineOf(text(t, resp), "body") != "dict" {
		t.Fatal("keywords only")
	}
}

func mustURL(t *testing.T, s string) *neturl.URL {
	t.Helper()
	u, err := urlvalue.Parse("test", s)
	if err != nil {
		t.Fatal(err)
	}
	return u.Go()
}

func TestArgumentErrors(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(echo))
	defer srv.Close()
	post := member(t, "post")
	cases := []struct {
		f    *object.Function
		pos  []object.Object
		kws  []object.KwArg
		kind *object.Error
		msg  string
	}{
		{post, []object.Object{str(srv.URL)}, []object.KwArg{kw("body", str("a")), kw("json", object.Integer(1))}, object.ValueError, "post() takes at most one of 'body', 'json' and 'form'"},
		{post, []object.Object{str(srv.URL)}, []object.KwArg{kw("headers", dict(str("content-length"), str("3")))}, object.ValueError, "post() argument 'headers' must not set Content-Length; the library frames the body"},
		{post, []object.Object{str(srv.URL)}, []object.KwArg{kw("headers", dict(str("Transfer-Encoding"), str("chunked")))}, object.ValueError, "must not set Transfer-Encoding"},
		{post, []object.Object{str(srv.URL)}, []object.KwArg{kw("headers", dict(str("X"), object.Integer(1)))}, object.TypeError, "post() argument 'headers' values must be String or List of String, got Int"},
		{post, []object.Object{str(srv.URL)}, []object.KwArg{kw("body", object.Integer(1))}, object.TypeError, "post() argument 'body' must be String, Bytes, Nil or have a read(size) method, got Int"},
		{post, []object.Object{str(srv.URL)}, []object.KwArg{kw("params", dict(str("a"), dict()))}, object.TypeError, "post() argument 'params' values must be"},
		{post, []object.Object{str("users/7")}, nil, object.ValueError, `post() needs an absolute URL, got "users/7"`},
		{post, []object.Object{str("ftp://example.com/x")}, nil, object.ValueError, `post() URL scheme must be http or https, got "ftp"`},
		{post, []object.Object{str("ht!tp://x")}, nil, object.ParseError, `post() invalid URL "ht!tp://x": first path segment in URL cannot contain colon`},
		{post, []object.Object{object.Integer(1)}, nil, object.TypeError, "post() argument 'url' must be String or URL, got Int"},
		{member(t, "request"), []object.Object{str("GE T"), str(srv.URL)}, nil, object.ValueError, `request() invalid method "GE T"`},
		{post, []object.Object{str(srv.URL)}, []object.KwArg{kw("timeout", object.Integer(-1))}, object.ValueError, "post() argument 'timeout' must be non-negative"},
	}
	for _, c := range cases {
		_, err := callErr(t, c.f, c.pos, c.kws...)
		if err == nil || !errors.Is(err, c.kind) || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%s(%v, %v) = %v, want %s %q", c.f.Name, c.pos, c.kws, err, c.kind, c.msg)
		}
	}
	_, err := callErr(t, post, []object.Object{str("ht!tp://x")})
	if field(t, err, "text") != str("ht!tp://x") {
		t.Fatal("invalid URL text data")
	}
}

func TestStatusErrorAndCheck(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(stdhttp.StatusNotFound)
		io.WriteString(w, `{"error": "no such user"}`)
	}))
	defer srv.Close()
	f := member(t, "get")
	_, err := callErr(t, f, []object.Object{str(srv.URL + "/users/9")}, kw("params", dict(str("v"), str("2"))))
	if !errors.Is(err, StatusError) || errors.Is(err, object.IOError) {
		t.Fatalf("error = %v", err)
	}
	if want := "get() " + srv.URL + "/users/9?v=2 returned 404 Not Found"; err.Error() != want {
		t.Fatalf("message = %q, want %q", err.Error(), want)
	}
	if field(t, err, "status") != object.Integer(404) {
		t.Fatal("status data")
	}
	resp := field(t, err, "response")
	if get(t, resp, "closed") != object.False {
		t.Fatal("the response of a StatusError reads as unread")
	}
	if got := method(t, resp, "json", nil).(*object.Dict); got.Size() != object.Integer(1) {
		t.Fatal("e.response.json()")
	}
	if got := get(t, resp, "headers").(*object.Dict); !modtest.Equal(mustGet(t, got, "retry-after"), str("120")) {
		t.Fatal("e.response.headers")
	}
	if _, err := methodErr(t, resp, "text", nil); err == nil || err.Error() != "text() response body was already read" {
		t.Fatalf("second read = %v", err)
	}

	resp = call(t, f, []object.Object{str(srv.URL)}, kw("check", object.False))
	if get(t, resp, "status") != object.Integer(404) || get(t, resp, "success") != object.False {
		t.Fatal("check=false")
	}
	method(t, resp, "close", nil)
}

func has(d *object.Dict, key string) bool {
	_, ok, _ := d.Get(str(key))
	return ok
}

func mustGet(t *testing.T, d *object.Dict, key string) object.Object {
	t.Helper()
	v, ok, err := d.Get(str(key))
	if err != nil || !ok {
		t.Fatalf("missing %q in %v", key, d)
	}
	return v
}

func TestResponseReading(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Header().Add("Set-Cookie", "a=1; Path=/; Expires=Wed, 21 Oct 2037 07:28:00 GMT")
		w.Header().Add("Set-Cookie", "b=2")
		w.Header().Add("X-Many", "one")
		w.Header().Add("X-Many", "two")
		switch r.URL.Path {
		case "/bad-utf8":
			w.Write([]byte("ok\r\nbad\xff\n"))
		case "/notjson":
			io.WriteString(w, "{\n  \"a\": }")
		default:
			io.WriteString(w, "first\r\nsecond\n\nlast")
		}
	}))
	defer srv.Close()
	f := member(t, "get")

	resp := call(t, f, []object.Object{str(srv.URL)})
	headers := get(t, resp, "headers").(*object.Dict)
	if !modtest.Equal(mustGet(t, headers, "x-many"), str("one, two")) || get(t, resp, "headers") != object.Object(headers) {
		t.Fatalf("headers = %v", headers)
	}
	cookies := method(t, resp, "header_values", []object.Object{str("SET-COOKIE")})
	if !modtest.Equal(cookies, list(str("a=1; Path=/; Expires=Wed, 21 Oct 2037 07:28:00 GMT"), str("b=2"))) {
		t.Fatalf("header_values = %v", cookies)
	}
	if !modtest.Equal(method(t, resp, "header_values", []object.Object{str("x-absent")}), list()) {
		t.Fatal("absent header_values")
	}
	if _, err := methodErr(t, resp, "header_values", []object.Object{object.Integer(1)}); !errors.Is(err, object.TypeError) {
		t.Fatal("header_values(1)")
	}
	if got := method(t, resp, "read", []object.Object{object.Integer(3)}); string(got.(object.Bytes)) != "fir" {
		t.Fatalf("read(3) = %v", got)
	}
	lines, err := object.Collect(resp)
	if err != nil || !modtest.Equal(&object.List{Elements: lines}, list(str("st"), str("second"), str(""), str("last"))) {
		t.Fatalf("lines = %v, %v", lines, err)
	}
	if get(t, resp, "closed") != object.True {
		t.Fatal("a body read to its end is closed")
	}
	if got := method(t, resp, "read", nil); len(got.(object.Bytes)) != 0 {
		t.Fatal("read() after the end")
	}
	if _, err := methodErr(t, resp, "text", nil); err == nil || err.Error() != "text() response body was already read" {
		t.Fatalf("text() after end = %v", err)
	}
	method(t, resp, "close", nil)
	method(t, resp, "close", nil)
	if _, err := methodErr(t, resp, "read", nil); !errors.Is(err, object.ValueError) {
		t.Fatal("read() after close()")
	}

	resp = call(t, f, []object.Object{str(srv.URL + "/bad-utf8")})
	_, err = object.Collect(resp)
	if !errors.Is(err, object.ValueError) || err.Error() != "line 2 of the response body is not valid UTF-8" || field(t, err, "line") != object.Integer(2) {
		t.Fatalf("bad line = %v", err)
	}
	resp = call(t, f, []object.Object{str(srv.URL + "/bad-utf8")})
	if _, err := methodErr(t, resp, "text", nil); err == nil || err.Error() != "text() response body is not valid UTF-8" {
		t.Fatalf("text() bad = %v", err)
	}
	resp = call(t, f, []object.Object{str(srv.URL + "/notjson")})
	_, err = methodErr(t, resp, "json", nil)
	if !errors.Is(err, object.ParseError) || field(t, err, "line") != object.Integer(2) || field(t, err, "column") == nil {
		t.Fatalf("json() = %v", err)
	}

	// A Response is a readable: anything that takes data takes it.
	resp = call(t, f, []object.Object{str(srv.URL)})
	r, ok := object.DataReader("test", "data", resp)
	if !ok {
		t.Fatal("Response is not readable")
	}
	all, _ := io.ReadAll(r)
	if string(all) != "first\r\nsecond\n\nlast" {
		t.Fatalf("read through DataReader = %q", all)
	}
	if fmt.Sprint(resp) != "<Response 200 OK>" || resp.(*Response).Constructor() != responseType.Function {
		t.Fatalf("Show = %v", resp)
	}
}

func TestRedirects(t *testing.T) {
	var hits atomic.Int32
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("/short", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		stdhttp.Redirect(w, r, "/landed?from=short", stdhttp.StatusFound)
	})
	mux.HandleFunc("/keep", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		hits.Add(1)
		stdhttp.Redirect(w, r, "/echo", stdhttp.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/landed", func(w stdhttp.ResponseWriter, r *stdhttp.Request) { io.WriteString(w, "landed") })
	mux.HandleFunc("/echo", echo)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp := call(t, member(t, "get"), []object.Object{str(srv.URL + "/short")})
	u := get(t, resp, "url")
	if fmt.Sprint(u) != srv.URL+"/landed?from=short" || get(t, u, "path") != str("/landed") || text(t, resp) != "landed" {
		t.Fatalf("url = %v", u)
	}

	// 307 with a body that can be sent again: followed, same method and body.
	body := text(t, call(t, member(t, "post"), []object.Object{str(srv.URL + "/keep")}, kw("body", str("again"))))
	if lineOf(body, "method") != "POST" || lineOf(body, "body") != "again" {
		t.Fatalf("307 with bytes: %s", body)
	}
	// 307 with a stream: Go returns the 307 itself, readable, not an error.
	path := filepath.Join(t.TempDir(), "up.txt")
	os.WriteFile(path, []byte("stream"), 0o644)
	file, _ := os.Open(path)
	f := filestream.Open(path, file, false)
	defer f.Close(object.CallArgs{})
	resp = call(t, member(t, "post"), []object.Object{str(srv.URL + "/keep")}, kw("body", f))
	if get(t, resp, "status") != object.Integer(307) || !modtest.Equal(mustGet(t, get(t, resp, "headers").(*object.Dict), "location"), str("/echo")) {
		t.Fatalf("307 with a stream = %v", resp)
	}
	text(t, resp)

	// Client(redirects=false) returns the 3xx as it is, without raising.
	client := call(t, clientType.Function, nil, kw("redirects", object.False))
	resp = method(t, client, "get", []object.Object{str(srv.URL + "/short")})
	if get(t, resp, "status") != object.Integer(302) || get(t, resp, "success") != object.True || fmt.Sprint(get(t, resp, "url")) != srv.URL+"/short" {
		t.Fatalf("redirects=false = %v %v", get(t, resp, "status"), get(t, resp, "url"))
	}
	if !modtest.Equal(mustGet(t, get(t, resp, "headers").(*object.Dict), "location"), str("/landed?from=short")) {
		t.Fatal("location header")
	}
}

func TestCookies(t *testing.T) {
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("/login", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		stdhttp.SetCookie(w, &stdhttp.Cookie{Name: "session", Value: "s3cr3t", Path: "/"})
		stdhttp.SetCookie(w, &stdhttp.Cookie{Name: "csrftoken", Value: "tok", Path: "/"})
		stdhttp.Redirect(w, r, "/account", stdhttp.StatusFound)
	})
	mux.HandleFunc("/account", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if c, err := r.Cookie("session"); err != nil || c.Value != "s3cr3t" {
			stdhttp.Error(w, "log in first", stdhttp.StatusUnauthorized)
			return
		}
		io.WriteString(w, "welcome")
	})
	mux.HandleFunc("/echo", echo)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// A module call keeps cookies along its own redirect chain, and forgets
	// them when it returns.
	if text(t, call(t, member(t, "post"), []object.Object{str(srv.URL + "/login")}, kw("form", dict(str("user"), str("ada"))))) != "welcome" {
		t.Fatal("redirect chain lost the cookie")
	}
	if _, err := callErr(t, member(t, "get"), []object.Object{str(srv.URL + "/account")}); !errors.Is(err, StatusError) {
		t.Fatalf("a module call kept cookies: %v", err)
	}

	client := call(t, clientType.Function, nil)
	method(t, client, "post", []object.Object{str(srv.URL + "/login")})
	if text(t, method(t, client, "get", []object.Object{str(srv.URL + "/account")})) != "welcome" {
		t.Fatal("client lost the session")
	}
	cookies := method(t, client, "cookies", []object.Object{str(srv.URL)})
	if !modtest.Equal(mustGet(t, cookies.(*object.Dict), "csrftoken"), str("tok")) || cookies.(*object.Dict).Size() != object.Integer(2) {
		t.Fatalf("cookies(url) = %v", cookies)
	}
	if got := method(t, client, "cookies", []object.Object{str("http://other.example/")}); got.(*object.Dict).Size() != object.Integer(0) {
		t.Fatal("cookies of another host")
	}
	// A Cookie header passed by the caller is kept; the jar's are added.
	body := text(t, method(t, client, "get", []object.Object{str(srv.URL + "/echo")}, kw("headers", dict(str("Cookie"), str("extra=1")))))
	if c := lineOf(body, "cookie"); !strings.HasPrefix(c, "extra=1; ") || !strings.Contains(c, "session=s3cr3t") {
		t.Fatalf("Cookie header = %q", c)
	}

	// A saved session restored by URL.
	resumed := call(t, clientType.Function, nil, kw("cookies", dict(str(srv.URL), dict(str("session"), str("s3cr3t")))))
	if text(t, method(t, resumed, "get", []object.Object{str(srv.URL + "/account")})) != "welcome" {
		t.Fatal("Client(cookies=) did not start the session")
	}
	if _, err := callErr(t, clientType.Function, nil, kw("cookies", dict(str("session"), str("s3cr3t")))); err == nil || err.Error() != "Client() argument 'cookies' values must be Dict, got String" {
		t.Fatalf("flat cookies = %v", err)
	}
	if _, err := methodErr(t, client, "cookies", []object.Object{str("ht!tp://x")}); !errors.Is(err, object.ParseError) {
		t.Fatal("cookies(invalid URL)")
	}
}

func TestClientDefaults(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path == "/slow" {
			time.Sleep(300 * time.Millisecond)
		}
		echo(w, r)
	}))
	defer srv.Close()
	client := call(t, clientType.Function, nil, kw("headers", dict(str("X-Token"), str("client"), str("Accept"), str("text/plain"))), kw("timeout", object.Float(0.1)))
	body := text(t, method(t, client, "get", []object.Object{str(srv.URL)}, kw("headers", dict(str("x-token"), str("call")))))
	if lineOf(body, "x-token") != "call" {
		t.Fatalf("per-call header did not replace the client's: %s", body)
	}
	_, err := methodErr(t, client, "get", []object.Object{str(srv.URL + "/slow")})
	if !errors.Is(err, object.TimeoutError) || err.Error() != "get() timed out after 0.1 seconds" {
		t.Fatalf("client timeout = %v", err)
	}
	// A per-call timeout replaces the client's, nil included.
	method(t, client, "get", []object.Object{str(srv.URL + "/slow")}, kw("timeout", object.Nil))
	if fmt.Sprint(client) != "<Client>" {
		t.Fatal("Show")
	}
}

func TestTimeoutsAndNetworkErrors(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path == "/stall-body" {
			io.WriteString(w, "start\n")
			w.(stdhttp.Flusher).Flush()
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	_, err := callErr(t, member(t, "get"), []object.Object{str(srv.URL)}, kw("timeout", object.Float(0.2)))
	if !errors.Is(err, object.TimeoutError) || !errors.Is(err, object.IOError) || errors.Is(err, object.NetworkError) {
		t.Fatalf("timeout = %v", err)
	}
	// The timeout covers reading the body.
	resp := call(t, member(t, "get"), []object.Object{str(srv.URL + "/stall-body")}, kw("timeout", object.Float(0.3)))
	_, err = object.Collect(resp)
	if !errors.Is(err, object.TimeoutError) {
		t.Fatalf("body timeout = %v", err)
	}

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	closed := "http://" + ln.Addr().String() + "/"
	ln.Close()
	_, err = callErr(t, member(t, "get"), []object.Object{str(closed)})
	if !errors.Is(err, object.NetworkError) || !strings.HasPrefix(err.Error(), "get() cannot reach 127.0.0.1:") {
		t.Fatalf("closed port = %v", err)
	}
}

func TestLineIterationBreakReleasesOnClose(t *testing.T) {
	gone := make(chan struct{})
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		for i := 0; ; i++ {
			if _, err := fmt.Fprintf(w, "data: %d\n", i); err != nil {
				break
			}
			w.(stdhttp.Flusher).Flush()
			select {
			case <-r.Context().Done():
				close(gone)
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
		close(gone)
	}))
	defer srv.Close()
	resp := call(t, member(t, "get"), []object.Object{str(srv.URL)}, kw("timeout", object.Nil))
	it, err := object.Iterate(resp)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		v, ok, err := it.Next()
		if err != nil || !ok || v != str(fmt.Sprintf("data: %d", i)) {
			t.Fatalf("line %d = %v %v %v", i, v, ok, err)
		}
	}
	it.Close()
	if get(t, resp, "closed") != object.False {
		t.Fatal("break closed the response")
	}
	method(t, resp, "close", nil)
	select {
	case <-gone:
	case <-time.After(5 * time.Second):
		t.Fatal("close() did not release the connection")
	}
	if _, err := object.Iterate(resp); err == nil {
		t.Fatal("iterating a closed response")
	}
}

func TestGzipDecoding(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			io.WriteString(w, "plain")
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		io.WriteString(zw, "decoded\n")
		zw.Close()
	}))
	defer srv.Close()
	resp := call(t, member(t, "get"), []object.Object{str(srv.URL)})
	headers := get(t, resp, "headers").(*object.Dict)
	if text(t, resp) != "decoded\n" || has(headers, "content-encoding") {
		t.Fatalf("gzip: headers %v", headers)
	}
	// Your own Accept-Encoding: the body arrives as sent.
	resp = call(t, member(t, "get"), []object.Object{str(srv.URL)}, kw("headers", dict(str("Accept-Encoding"), str("gzip"))))
	raw := method(t, resp, "read", nil).(object.Bytes)
	if !bytes.HasPrefix(raw, []byte{0x1f, 0x8b}) {
		t.Fatalf("raw body = %q", raw)
	}
}

func TestInsecure(t *testing.T) {
	srv := httptest.NewUnstartedServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		stdhttp.SetCookie(w, &stdhttp.Cookie{Name: "seen", Value: "1", Path: "/", Secure: true})
		fmt.Fprintf(w, "proto=%s", r.Proto)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	_, err := callErr(t, member(t, "get"), []object.Object{str(srv.URL)})
	if !errors.Is(err, object.NetworkError) || !strings.HasPrefix(err.Error(), "get() cannot reach") {
		t.Fatalf("self-signed with the default client = %v", err)
	}
	if _, err := methodErr(t, call(t, clientType.Function, nil), "get", []object.Object{str(srv.URL)}); !errors.Is(err, object.NetworkError) {
		t.Fatalf("self-signed with a default Client = %v", err)
	}
	client := call(t, clientType.Function, nil, kw("insecure", object.True))
	resp := method(t, client, "get", []object.Object{str(srv.URL + "/a/b")})
	if got := text(t, resp); got != "proto=HTTP/2.0" {
		t.Fatalf("insecure client = %q (the clone lost HTTP/2)", got)
	}
	if tr := transport(true).(*stdhttp.Transport); tr.Proxy == nil {
		t.Fatal("the insecure transport lost the proxy setting")
	}
	// A Secure cookie for an https URL is sent to every path of the host,
	// and never over plain http.
	if got := method(t, client, "cookies", []object.Object{str(srv.URL + "/other/path")}).(*object.Dict); !has(got, "seen") {
		t.Fatalf("cookies(https) = %v", got)
	}
	// Go's jar treats loopback hosts as secure origins, as browsers do, so
	// the "never over http" half is checked on a name that is not one.
	plain := strings.Replace(srv.URL, "https://", "http://", 1)
	if got := method(t, client, "cookies", []object.Object{str(plain)}).(*object.Dict); !has(got, "seen") {
		t.Fatalf("cookies(http on loopback) = %v", got)
	}
	restored := call(t, clientType.Function, nil, kw("cookies", dict(str("https://shop.example.test"), dict(str("tls"), str("x")))))
	if got := method(t, restored, "cookies", []object.Object{str("https://shop.example.test/deep/path")}).(*object.Dict); !has(got, "tls") {
		t.Fatalf("restored https cookie = %v", got)
	}
	if got := method(t, restored, "cookies", []object.Object{str("http://shop.example.test/")}).(*object.Dict); got.Size() != object.Integer(0) {
		t.Fatal("a restored https cookie is sent over http")
	}
	if got := method(t, restored, "cookies", []object.Object{str("https://other.example.test/")}).(*object.Dict); got.Size() != object.Integer(0) {
		t.Fatal("a restored cookie is sent to another host")
	}
}

// upload parses a multipart form and reports what it received.
func upload(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		stdhttp.Error(w, err.Error(), stdhttp.StatusBadRequest)
		return
	}
	fmt.Fprintf(w, "content-length=%d\n", r.ContentLength)
	fmt.Fprintf(w, "transfer-encoding=%s\n", strings.Join(r.TransferEncoding, ","))
	for _, name := range []string{"title", "album"} {
		fmt.Fprintf(w, "%s=%s\n", name, strings.Join(r.MultipartForm.Value[name], "|"))
	}
	for _, name := range []string{"avatar", "report", "extra"} {
		for _, fh := range r.MultipartForm.File[name] {
			f, _ := fh.Open()
			data, _ := io.ReadAll(f)
			fmt.Fprintf(w, "file %s %s %s %s\n", name, fh.Filename, fh.Header.Get("Content-Type"), data)
		}
	}
}

func goblinFile(t *testing.T, path string) *filestream.File {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return filestream.Open(path, file, false)
}

func TestMultipartUpload(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(upload))
	defer srv.Close()
	dir := t.TempDir()
	photo := filepath.Join(dir, "photo.png")
	content := bytes.Repeat([]byte("PNG"), 50000)
	os.WriteFile(photo, content, 0o644)
	f := goblinFile(t, photo)
	defer f.Close(object.CallArgs{})

	result := call(t, member(t, "multipart"), nil,
		kw("fields", dict(str("title"), str("Me"), str("album"), list(str("2026"), object.Integer(7)))),
		kw("files", dict(
			str("avatar"), f,
			str("report"), dict(str("name"), str("report.csv"), str("data"), str("a,b\n"), str("content_type"), str("text/csv")),
			str("extra"), list(dict(str("data"), object.Bytes("one"), str("name"), str("1.bin")), dict(str("data"), object.Bytes("two"), str("name"), str("2.bin"))),
		)))
	parts := result.(*object.List).Elements
	body, contentType := parts[0], string(parts[1].(object.String))
	if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") || fmt.Sprint(body) != "<File multipart>" || get(t, body, "path") != object.Nil {
		t.Fatalf("multipart() = %v %q", body, contentType)
	}
	resp := call(t, member(t, "post"), []object.Object{str(srv.URL)}, kw("body", body), kw("headers", dict(str("Content-Type"), str(contentType))))
	got := text(t, resp)
	want := []string{
		"transfer-encoding=\n",
		"title=Me\n", "album=2026|7\n",
		"file avatar photo.png application/octet-stream " + string(content) + "\n",
		"file report report.csv text/csv a,b\n",
		"file extra 1.bin application/octet-stream one\n", "file extra 2.bin application/octet-stream two\n",
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Fatalf("server saw %.300q, missing %q", got, w)
		}
	}
	if lineOf(got, "content-length") == "-1" {
		t.Fatal("a body of known parts was sent chunked")
	}
	// The library closed nothing: the caller's file is open, at its end.
	if get(t, f, "closed") != object.False {
		t.Fatal("the caller's file was closed")
	}
	// The body is good for one request.
	_, err := callErr(t, member(t, "post"), []object.Object{str(srv.URL)}, kw("body", body))
	if err == nil || err.Error() != "post() this multipart body was already sent; build it again" {
		t.Fatalf("second send = %v", err)
	}

	// A part whose length is unknown makes the body chunked.
	unknown := filestream.NewReader("pipe", strings.NewReader("piped"), nil)
	result = call(t, member(t, "multipart"), nil, kw("files", dict(str("avatar"), dict(str("data"), unknown, str("name"), str("p.txt")))))
	parts = result.(*object.List).Elements
	got = text(t, call(t, member(t, "post"), []object.Object{str(srv.URL)}, kw("body", parts[0]), kw("headers", dict(str("Content-Type"), parts[1]))))
	if lineOf(got, "transfer-encoding") != "chunked" || !strings.Contains(got, "file avatar p.txt application/octet-stream piped") {
		t.Fatalf("chunked multipart: %q", got)
	}

	// An empty form is valid.
	result = call(t, member(t, "multipart"), nil)
	parts = result.(*object.List).Elements
	resp = call(t, member(t, "post"), []object.Object{str(srv.URL)}, kw("body", parts[0]), kw("headers", dict(str("Content-Type"), parts[1])))
	if get(t, resp, "status") != object.Integer(200) {
		t.Fatal("empty form")
	}
}

func TestMultipartArgumentErrors(t *testing.T) {
	stdin := filestream.NewReader("stdin", strings.NewReader(""), nil)
	cases := []struct {
		files object.Object
		kind  *object.Error
		msg   string
	}{
		{dict(str("avatar"), str("photo.png")), object.ValueError, `multipart() file "avatar" needs a name: pass an open fs.File, or {"name": …, "data": …}`},
		{dict(str("avatar"), object.Bytes("x")), object.ValueError, "needs a name"},
		{dict(str("avatar"), stdin), object.ValueError, "needs a name"},
		{dict(str("avatar"), dict(str("data"), str("x"))), object.ValueError, "needs a name"},
		{dict(str("avatar"), dict(str("name"), str("a"))), object.ValueError, `multipart() file "avatar" needs "data"`},
		{dict(str("avatar"), dict(str("data"), str("x"), str("name"), str("a"), str("type"), str("t"))), object.ValueError, `multipart() file "avatar" has an unknown key "type"`},
		{dict(str("avatar"), dict(str("data"), object.Integer(1), str("name"), str("a"))), object.TypeError, `multipart() file "avatar" data must be String, Bytes or have a read(size) method, got Int`},
		{dict(str("avatar"), object.Integer(1)), object.TypeError, "multipart() argument 'files' values must be File, Dict or List, got Int"},
		{str("x"), object.TypeError, "multipart() argument 'files' must be Dict or Nil, got String"},
	}
	for _, c := range cases {
		_, err := callErr(t, member(t, "multipart"), nil, kw("files", c.files))
		if err == nil || !errors.Is(err, c.kind) || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("multipart(files=%v) = %v, want %q", c.files, err, c.msg)
		}
	}
	if _, err := callErr(t, member(t, "multipart"), nil, kw("fields", dict(str("a"), dict()))); !errors.Is(err, object.TypeError) {
		t.Errorf("fields with a Dict value = %v", err)
	}
}

// lying is a body that claims a length it does not have.
type lying struct {
	io.Reader
	n int64
}

func (l lying) Remaining() (int64, bool) { return l.n, true }

func TestStreamedBodies(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(echo))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "backup.tar.gz")
	os.WriteFile(path, []byte("0123456789"), 0o644)
	f := goblinFile(t, path)
	method(t, f, "read", []object.Object{object.Integer(3)})
	got := text(t, call(t, member(t, "put"), []object.Object{str(srv.URL)}, kw("body", f), kw("timeout", object.Nil)))
	if lineOf(got, "content-length") != "7" || lineOf(got, "body") != "3456789" {
		t.Fatalf("file body from its position: %s", got)
	}
	if get(t, f, "closed") != object.False {
		t.Fatal("the body's file was closed")
	}
	// At its end the file sends an empty body.
	got = text(t, call(t, member(t, "put"), []object.Object{str(srv.URL)}, kw("body", f)))
	if lineOf(got, "content-length") != "0" || lineOf(got, "body") != "" {
		t.Fatalf("exhausted file: %s", got)
	}

	// A readable of unknown length is chunked.
	pipe := filestream.NewReader("pipe", strings.NewReader("piped"), nil)
	got = text(t, call(t, member(t, "post"), []object.Object{str(srv.URL)}, kw("body", pipe)))
	if lineOf(got, "transfer-encoding") != "chunked" || lineOf(got, "body") != "piped" {
		t.Fatalf("chunked: %s", got)
	}

	// A readable that raises: the request is abandoned and its error comes back unchanged.
	raising := &object.Module{Name: "broken", Members: map[string]object.Object{
		"read": &object.Function{Name: "read", Fn: func(object.CallArgs) (object.Object, error) {
			return nil, object.NewKindError(object.KeyError, "the source broke").With("path", str("src"))
		}},
	}}
	_, err := callErr(t, member(t, "post"), []object.Object{str(srv.URL)}, kw("body", raising))
	if !errors.Is(err, object.KeyError) || !strings.Contains(err.Error(), "the source broke") || field(t, err, "path") != str("src") {
		t.Fatalf("raising body = %v", err)
	}
	// A closed File raises its ValueError.
	method(t, f, "close", nil)
	_, err = callErr(t, member(t, "post"), []object.Object{str(srv.URL)}, kw("body", f))
	if !errors.Is(err, object.ValueError) || !strings.Contains(err.Error(), "closed file") {
		t.Fatalf("closed file body = %v", err)
	}

	// A declared length that does not match the data is a NetworkError.
	for _, l := range []lying{{strings.NewReader("short"), 100}, {strings.NewReader("much longer"), 2}} {
		body := filestream.NewReader("liar", l, nil)
		_, err := callErr(t, member(t, "post"), []object.Object{str(srv.URL)}, kw("body", body))
		if !errors.Is(err, object.NetworkError) || !strings.HasPrefix(err.Error(), "post() the body changed size while it was sent to 127.0.0.1:") {
			t.Fatalf("length %d over %T = %v", l.n, l.Reader, err)
		}
	}
}
