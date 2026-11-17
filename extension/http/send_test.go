package http

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"sync"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/extension/internal/urlvalue"
	"github.com/aisk/goblin/object"
)

// dumper answers with the request exactly as the server read it, minus the
// parts that differ between connections.
func dumper(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	dump, _ := httputil.DumpRequest(r, true)
	w.Write(dump)
}

// TestEquations: get, request and send build the same request at the
// server for the same inputs, on the module and on a Client.
func TestEquations(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(dumper))
	defer srv.Close()
	h := func() object.Object {
		return dict(str("X-A"), list(str("1"), str("2")), str("Accept"), str("text/plain"))
	}
	for _, on := range []string{"module", "client"} {
		fn := func(name string) *object.Function {
			if on == "module" {
				return member(t, name)
			}
			return get(t, newClientT(t), name).(*object.Function)
		}
		viaGet := text(t, call(t, fn("get"), args(str(srv.URL+"/p?q=1")), kw("headers", h()), kw("timeout", object.Integer(5)), kw("check", object.True)))
		viaRequest := text(t, call(t, fn("request"), args(str("GET"), str(srv.URL+"/p?q=1")), kw("headers", h()), kw("timeout", object.Integer(5))))
		viaSend := text(t, call(t, fn("send"), args(newRequestT(t, "GET", srv.URL+"/p?q=1", kw("headers", h()))), kw("timeout", object.Integer(5))))
		if viaGet != viaRequest || viaRequest != viaSend {
			t.Fatalf("%s:\nget:\n%s\nrequest:\n%s\nsend:\n%s", on, viaGet, viaRequest, viaSend)
		}
		posted := text(t, call(t, fn("request"), args(str("POST"), str(srv.URL), h(), str("body"))))
		sent := text(t, call(t, fn("send"), args(newRequestT(t, "POST", srv.URL, kw("headers", h()), kw("body", str("body"))))))
		if posted != sent {
			t.Fatalf("%s: request %q != send %q", on, posted, sent)
		}
	}
	// The positional order of request is method, url, headers, body.
	got := text(t, call(t, member(t, "request"), args(str("PUT"), str(srv.URL), dict(str("X-P"), str("1")), str("payload"), object.Integer(5), object.False)))
	if !strings.Contains(got, "X-P: 1") || !strings.HasSuffix(got, "payload") {
		t.Fatalf("positional request = %q", got)
	}
}

// TestSendUserAgentBehaviours: each step 1 addition and its off switch.
func TestSendUserAgentBehaviours(t *testing.T) {
	rec := &recorder{next: func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.URL.Path {
		case "/hop":
			stdhttp.Redirect(w, r, "/end", stdhttp.StatusFound)
		default:
			io.WriteString(w, "ok")
		}
	}}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	// Nothing but User-Agent and Accept-Encoding is added to a plain get.
	call(t, member(t, "get"), args(str(srv.URL)))
	got := rec.last(t)
	sameNames(t, "get", got.header, "User-Agent", "Accept-Encoding")
	if got.header.Get("User-Agent") != "goblin/"+object.Version || got.header.Get("Accept-Encoding") != "gzip" {
		t.Fatalf("defaults = %v", got.header)
	}
	// A caller's User-Agent wins, from the request or the Client.
	call(t, member(t, "get"), args(str(srv.URL)), kw("headers", dict(str("user-agent"), str("mine/1"))))
	if ua := rec.last(t).header.Values("User-Agent"); len(ua) != 1 || ua[0] != "mine/1" {
		t.Fatalf("caller's User-Agent = %v", ua)
	}
	client := newClientT(t, kw("headers", dict(str("User-Agent"), str("client/2"), str("X-Default"), str("d"), str("Authorization"), str("Bearer c"))))
	method(t, client, "get", args(str(srv.URL)))
	got = rec.last(t)
	if got.header.Get("User-Agent") != "client/2" || got.header.Get("X-Default") != "d" || got.header.Get("Authorization") != "Bearer c" {
		t.Fatalf("client defaults = %v", got.header)
	}
	// A default header is added only when the request does not name it, in
	// any case.
	method(t, client, "get", args(str(srv.URL)), kw("headers", dict(str("x-default"), str("mine"))))
	if v := rec.last(t).header.Values("X-Default"); len(v) != 1 || v[0] != "mine" {
		t.Fatalf("request header beside a default = %v", v)
	}
	// Accept-Encoding: not with the caller's own, a Range header, or HEAD.
	call(t, member(t, "get"), args(str(srv.URL)), kw("headers", dict(str("Accept-Encoding"), str("identity"))))
	if v := rec.last(t).header.Values("Accept-Encoding"); len(v) != 1 || v[0] != "identity" {
		t.Fatalf("caller's Accept-Encoding = %v", v)
	}
	call(t, member(t, "get"), args(str(srv.URL)), kw("headers", dict(str("Range"), str("bytes=0-1"))))
	if _, ok := rec.last(t).header["Accept-Encoding"]; ok {
		t.Fatal("gzip asked for with a Range header")
	}
	call(t, member(t, "request"), args(str("HEAD"), str(srv.URL)))
	if _, ok := rec.last(t).header["Accept-Encoding"]; ok {
		t.Fatal("gzip asked for on HEAD")
	}
	method(t, newClientT(t, kw("headers", dict(str("Accept-Encoding"), str("identity")))), "get", args(str(srv.URL)))
	if v := rec.last(t).header.Get("Accept-Encoding"); v != "identity" {
		t.Fatalf("client's Accept-Encoding = %v", v)
	}
	// No Referer reaches the second hop; a caller's own does, unchanged.
	call(t, member(t, "get"), args(str(srv.URL+"/hop?secret=1")))
	end := rec.last(t)
	if end.path != "/end" {
		t.Fatal("redirect not followed")
	}
	sameNames(t, "second hop", end.header, "User-Agent", "Accept-Encoding")
	call(t, member(t, "get"), args(str(srv.URL+"/hop")), kw("headers", dict(str("Referer"), str("https://origin.test/page"))))
	if v := rec.last(t).header.Values("Referer"); len(v) != 1 || v[0] != "https://origin.test/page" {
		t.Fatalf("caller's Referer at the second hop = %v", v)
	}
	// No Content-Type is derived from a body.
	call(t, member(t, "request"), args(str("POST"), str(srv.URL)), kw("body", str(`{"a": 1}`)))
	if _, ok := rec.last(t).header["Content-Type"]; ok {
		t.Fatal("a Content-Type was added")
	}
}

func gzipHandler(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Header.Get("Accept-Encoding") != "gzip" {
		io.WriteString(w, "plain")
		return
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	io.WriteString(zw, "decoded line 1\ndecoded line 2\n")
	zw.Close()
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Length", itoa(buf.Len()))
	w.Write(buf.Bytes())
}

func TestSendGzip(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(gzipHandler))
	defer srv.Close()
	resp := call(t, member(t, "get"), args(str(srv.URL)))
	headers := get(t, resp, "headers").(*Headers)
	if headers.has("content-encoding") || headers.has("content-length") {
		t.Fatalf("decoded response headers = %v", headers)
	}
	if text(t, resp) != "decoded line 1\ndecoded line 2\n" {
		t.Fatal("not decoded")
	}
	// Line by line, as it is read.
	resp = call(t, member(t, "get"), args(str(srv.URL)))
	lines, err := object.Collect(body(t, resp))
	if err != nil || !modtest.Equal(list(lines...), list(str("decoded line 1"), str("decoded line 2"))) {
		t.Fatalf("lines = %v %v", lines, err)
	}
	// With the caller's own Accept-Encoding the body arrives as sent.
	resp = call(t, member(t, "get"), args(str(srv.URL)), kw("headers", dict(str("Accept-Encoding"), str("gzip"))))
	raw := method(t, body(t, resp), "read", nil).(object.Bytes)
	if !bytes.HasPrefix(raw, []byte{0x1f, 0x8b}) || !get(t, resp, "headers").(*Headers).has("content-encoding") {
		t.Fatalf("raw = %q", raw)
	}
	// Damaged gzip data: NetworkError from the read, the gzip error as cause.
	bad := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Write([]byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 0xff, 1, 2, 3, 4, 5, 6})
	}))
	defer bad.Close()
	resp = call(t, member(t, "get"), args(str(bad.URL)))
	_, err = methodErr(t, body(t, resp), "read", nil)
	host := strings.TrimPrefix(bad.URL, "http://")
	wantErrPrefix(t, err, object.NetworkError, "cannot read the response body from "+host+": flate: corrupt input")
	// An empty gzip-labelled body is an empty stream.
	empty := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(204)
	}))
	defer empty.Close()
	if text(t, call(t, member(t, "get"), args(str(empty.URL)))) != "" {
		t.Fatal("empty gzip body")
	}
}

// TestSendEqualsItsSteps: following a redirect chain with cookies by hand,
// with exchange, reaches what send reaches.
func TestSendEqualsItsSteps(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.URL.Path {
		case "/a":
			stdhttp.SetCookie(w, &stdhttp.Cookie{Name: "step", Value: "a", Path: "/"})
			stdhttp.Redirect(w, r, "/b", stdhttp.StatusFound)
		case "/b":
			if c, err := r.Cookie("step"); err != nil || c.Value != "a" {
				stdhttp.Error(w, "no cookie", 401)
				return
			}
			stdhttp.Redirect(w, r, "/c", stdhttp.StatusSeeOther)
		case "/c":
			fmt.Fprintf(w, "arrived with %s", r.Header.Get("Cookie"))
		}
	}))
	defer srv.Close()
	sent := call(t, member(t, "send"), args(newRequestT(t, "GET", srv.URL+"/a")))

	// By hand: one client for the cookie store, exchange for each hop.
	client := newClientT(t)
	target := srv.URL + "/a"
	var reply object.Object
	for {
		cookies := method(t, client, "cookies", args(str(target))).(*object.Dict)
		h := dict(str("User-Agent"), str(userAgent), str("Accept-Encoding"), str("gzip"))
		var pairs []string
		for _, e := range cookies.Entries() {
			pairs = append(pairs, e.Key.(object.String).String()+"="+e.Value.(object.String).String())
		}
		if len(pairs) > 0 {
			h.Set(str("Cookie"), str(strings.Join(pairs, "; ")))
		}
		req := newRequestT(t, "GET", target, kw("headers", h))
		reply = method(t, client, "exchange", args(req))
		for _, c := range method(t, get(t, reply, "headers"), "get_all", args(str("set-cookie"))).(*object.List).Elements {
			name, value, _ := strings.Cut(strings.Split(string(c.(object.String)), ";")[0], "=")
			// Step 3 by hand: the client's store keeps what the server set.
			client.(*Client).jar.SetCookies(get(t, req, "url").(*urlvalue.URL).Go(), []*stdhttp.Cookie{{Name: name, Value: value, Path: "/"}})
		}
		status := int(get(t, reply, "status").(object.Integer))
		if !isRedirect(status) {
			break
		}
		next := method(t, get(t, req, "url"), "join", args(method(t, get(t, reply, "headers"), "get", args(str("location")))))
		target = next.(interface{ String() string }).String()
		text(t, reply)
	}
	if get(t, reply, "status") != get(t, sent, "status") || !modtest.Equal(get(t, reply, "url"), get(t, sent, "url")) || text(t, reply) != text(t, sent) {
		t.Fatalf("by hand: %v %v; send: %v %v", get(t, reply, "status"), get(t, reply, "url"), get(t, sent, "status"), get(t, sent, "url"))
	}
}

// TestStatusCheck: StatusError and its data; check=false.
func TestStatusCheck(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.URL.Path {
		case "/teapot":
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(599)
			io.WriteString(w, "odd")
		case "/gone":
			if r.Header.Get("Accept-Encoding") == "gzip" {
				w.Header().Set("Content-Encoding", "gzip")
				w.WriteHeader(404)
				zw := gzip.NewWriter(w)
				io.WriteString(zw, `{"error": "no such user"}`)
				zw.Close()
				return
			}
			stdhttp.Error(w, "plain", 404)
		default:
			io.WriteString(w, "fine")
		}
	}))
	defer srv.Close()
	for _, fn := range []string{"get", "request", "send"} {
		var a []object.Object
		switch fn {
		case "get":
			a = args(str(srv.URL + "/gone"))
		case "request":
			a = args(str("GET"), str(srv.URL+"/gone"))
		case "send":
			a = args(newRequestT(t, "GET", srv.URL+"/gone"))
		}
		_, err := callErr(t, member(t, fn), a)
		wantErr(t, err, StatusError, fn+"() "+srv.URL+"/gone returned 404 Not Found")
		if field(t, err, "status") != object.Integer(404) {
			t.Fatal("status data")
		}
		resp := field(t, err, "response")
		if get(t, resp, "status") != object.Integer(404) || get(t, resp, "url").(interface{ String() string }).String() != srv.URL+"/gone" {
			t.Fatal("response data")
		}
		// The error body was read (and decoded) before the raise.
		if text(t, resp) != `{"error": "no such user"}` || get(t, get(t, resp, "headers"), "size") == object.Integer(0) {
			t.Fatalf("%s: e.response body", fn)
		}
		if errors.Is(err, object.IOError) {
			t.Fatal("StatusError is an IOError")
		}
		resp = call(t, member(t, fn), a, kw("check", object.False))
		if get(t, resp, "status") != object.Integer(404) {
			t.Fatal("check=false")
		}
	}
	_, err := callErr(t, member(t, "get"), args(str(srv.URL+"/teapot")))
	wantErr(t, err, StatusError, "get() "+srv.URL+"/teapot returned 599")
	resp := field(t, err, "response")
	if method(t, get(t, resp, "headers"), "get", args(str("retry-after"))) != str("7") || string(method(t, body(t, resp), "read", nil).(object.Bytes)) != "odd" {
		t.Fatal("599 response data")
	}
	// exchange never raises it.
	if get(t, call(t, member(t, "exchange"), args(newRequestT(t, "GET", srv.URL+"/gone"))), "status") != object.Integer(404) {
		t.Fatal("exchange raised")
	}
	if StatusError.Wrapped != object.BaseError {
		t.Fatal("StatusError's parent is not Error")
	}
}

// TestCookies: a Client keeps cookies across calls; a module call keeps
// them for its own redirect chain only; exchange neither stores nor sends.
func TestCookies(t *testing.T) {
	var mu sync.Mutex
	var lastCookie string
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		mu.Lock()
		lastCookie = r.Header.Get("Cookie")
		mu.Unlock()
		switch r.URL.Path {
		case "/login":
			stdhttp.SetCookie(w, &stdhttp.Cookie{Name: "session", Value: "s1", Path: "/"})
			stdhttp.Redirect(w, r, "/account", stdhttp.StatusSeeOther)
		case "/account":
			if c, err := r.Cookie("session"); err != nil || c.Value != "s1" {
				stdhttp.Error(w, "log in first", 401)
				return
			}
			io.WriteString(w, "hello")
		}
	}))
	defer srv.Close()
	cookie := func() string { mu.Lock(); defer mu.Unlock(); return lastCookie }

	// A module call keeps the cookie through its own redirect...
	if text(t, call(t, member(t, "request"), args(str("POST"), str(srv.URL+"/login")))) != "hello" {
		t.Fatal("module call lost the cookie on its redirect")
	}
	// ...and a second module call has none.
	_, err := callErr(t, member(t, "get"), args(str(srv.URL+"/account")))
	wantErrPrefix(t, err, StatusError, "get() ")
	if cookie() != "" {
		t.Fatalf("a module call sent %q", cookie())
	}
	// A Client keeps them.
	site := newClientT(t)
	text(t, method(t, site, "request", args(str("POST"), str(srv.URL+"/login"))))
	if text(t, method(t, site, "get", args(str(srv.URL+"/account")))) != "hello" {
		t.Fatal("client lost the session")
	}
	if got := method(t, site, "cookies", args(str(srv.URL))); !modtest.Equal(got, dict(str("session"), str("s1"))) {
		t.Fatalf("cookies() = %v", got)
	}
	// A Cookie header on the request is sent beside the kept ones.
	method(t, site, "get", args(str(srv.URL+"/account")), kw("headers", dict(str("Cookie"), str("extra=1"))))
	if c := cookie(); !strings.Contains(c, "extra=1") || !strings.Contains(c, "session=s1") {
		t.Fatalf("Cookie = %q", c)
	}
	// exchange on that Client sends none and stores none.
	resp := method(t, site, "exchange", args(newRequestT(t, "GET", srv.URL+"/account")))
	if get(t, resp, "status") != object.Integer(401) || cookie() != "" {
		t.Fatalf("exchange sent cookies: %q", cookie())
	}
	fresh := newClientT(t)
	method(t, fresh, "exchange", args(newRequestT(t, "POST", srv.URL+"/login")))
	if got := method(t, fresh, "cookies", args(str(srv.URL))).(*object.Dict); got.Size() != 0 {
		t.Fatalf("exchange stored %v", got)
	}
	// Restored cookies: per URL, every path, https only over TLS.
	restored := newClientT(t, kw("cookies", dict(str(srv.URL), dict(str("session"), str("s1")))))
	if text(t, method(t, restored, "get", args(str(srv.URL+"/account")))) != "hello" {
		t.Fatal("restored session")
	}
	tls := newClientT(t, kw("cookies", dict(str("https://shop.example.test"), dict(str("tls"), str("x")))))
	if got := method(t, tls, "cookies", args(str("https://shop.example.test/deep/path"))).(*object.Dict); got.Size() != 1 {
		t.Fatal("restored https cookie on another path")
	}
	if got := method(t, tls, "cookies", args(str("http://shop.example.test/"))).(*object.Dict); got.Size() != 0 {
		t.Fatal("a restored https cookie is sent over http")
	}
	if got := method(t, tls, "cookies", args(str("https://other.example.test/"))).(*object.Dict); got.Size() != 0 {
		t.Fatal("a restored cookie is sent to another host")
	}
	cases := []struct {
		cookies object.Object
		msg     string
	}{
		{dict(str("session"), str("abc")), "Client() argument 'cookies' values must be Dict, got String"},
		{dict(object.Integer(1), dict()), "Client() argument 'cookies' keys must be String or URL, got Int"},
		{dict(str("https://x.test"), dict(str("a"), object.Integer(1))), "Client() argument 'cookies' must map each URL to a Dict of String to String"},
		{list(), "Client() argument 'cookies' must be Dict or Nil, got List"},
	}
	for _, c := range cases {
		_, err := callErr(t, clientType.Function, nil, kw("cookies", c.cookies))
		wantErr(t, err, object.TypeError, c.msg)
	}
	_, err = methodErr(t, site, "cookies", args(str("ht!tp://x")))
	wantErrPrefix(t, err, object.ParseError, `cookies() invalid URL "ht!tp://x"`)
}
