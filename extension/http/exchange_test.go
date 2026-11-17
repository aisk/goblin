package http

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"net"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aisk/goblin/object"
)

// TestExchangeAddsNothing: the server receives exactly the request's
// headers, every status comes back, and the body is as the server sent it.
func TestExchangeAddsNothing(t *testing.T) {
	rec := &recorder{next: func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.URL.Path {
		case "/set":
			stdhttp.SetCookie(w, &stdhttp.Cookie{Name: "session", Value: "s1", Path: "/"})
			io.WriteString(w, "set")
		case "/moved":
			stdhttp.Redirect(w, r, "/elsewhere", stdhttp.StatusFound)
		case "/missing":
			stdhttp.Error(w, "nothing here", stdhttp.StatusNotFound)
		case "/gzip":
			w.Header().Set("Content-Encoding", "gzip")
			zw := gzip.NewWriter(w)
			io.WriteString(zw, "compressed")
			zw.Close()
		default:
			io.WriteString(w, "ok")
		}
	}}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	client := newClientT(t, kw("headers", dict(str("X-Default"), str("d"), str("User-Agent"), str("client/1"))))
	method(t, client, "get", args(str(srv.URL+"/set")))
	if method(t, client, "cookies", args(str(srv.URL))).(*object.Dict).Size() != 1 {
		t.Fatal("the client kept no cookie")
	}
	for _, exchange := range []*object.Function{member(t, "exchange"), get(t, client, "exchange").(*object.Function)} {
		req := newRequestT(t, "GET", srv.URL+"/plain", kw("headers", dict(str("X-Mine"), list(str("1"), str("2")))))
		resp := call(t, exchange, args(req))
		got := rec.last(t)
		sameNames(t, "exchange", got.header, "X-Mine")
		if strings.Join(got.header.Values("X-Mine"), "|") != "1|2" {
			t.Fatalf("values = %v", got.header)
		}
		if get(t, resp, "url").(object.Object).(interface{ String() string }).String() != srv.URL+"/plain" {
			t.Fatal("Response.url is not the request's URL")
		}
		// A request without headers sends none at all.
		call(t, exchange, args(newRequestT(t, "GET", srv.URL+"/plain")))
		sameNames(t, "exchange without headers", rec.last(t).header)
	}

	// A 302 and a 404 are returned as they are.
	moved := call(t, member(t, "exchange"), args(newRequestT(t, "GET", srv.URL+"/moved")))
	if get(t, moved, "status") != object.Integer(302) || method(t, get(t, moved, "headers"), "get", args(str("location"))) != str("/elsewhere") {
		t.Fatalf("302 = %v", moved)
	}
	missing := call(t, get(t, client, "exchange").(*object.Function), args(newRequestT(t, "GET", srv.URL+"/missing")))
	if get(t, missing, "status") != object.Integer(404) || text(t, missing) != "nothing here\n" {
		t.Fatal("404")
	}
	// A gzip body is the gzip bytes, content-encoding present; asked for
	// explicitly, the server compresses and exchange decodes nothing.
	zipped := call(t, member(t, "exchange"), args(newRequestT(t, "GET", srv.URL+"/gzip", kw("headers", dict(str("Accept-Encoding"), str("gzip"))))))
	raw := method(t, body(t, zipped), "read", nil).(object.Bytes)
	if !bytes.HasPrefix(raw, []byte{0x1f, 0x8b}) || method(t, get(t, zipped, "headers"), "get", args(str("content-encoding"))) != str("gzip") {
		t.Fatalf("gzip exchange = %q", raw)
	}
	// The request object is not changed.
	req := newRequestT(t, "GET", srv.URL+"/plain")
	call(t, member(t, "exchange"), args(req))
	if get(t, get(t, req, "headers"), "size") != object.Integer(0) {
		t.Fatal("exchange changed the request's headers")
	}
	// Argument errors.
	_, err := callErr(t, member(t, "exchange"), args(dict()))
	wantErr(t, err, object.TypeError, "exchange() argument 'request' must be Request, got Dict")
	_, err = callErr(t, member(t, "send"), args(str(srv.URL)))
	wantErr(t, err, object.TypeError, "send() argument 'request' must be Request, got String")
	_, err = callErr(t, member(t, "exchange"), args(req), kw("check", object.False))
	wantErrPrefix(t, err, object.TypeError, "exchange() got an unexpected keyword argument 'check'")
}

// TestExchangeAddsNothingHTTP2 checks the same over HTTP/2, where Go's
// encoder has its own defaults.
func TestExchangeAddsNothingHTTP2(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewUnstartedServer(rec)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	client := newClientT(t, kw("insecure", object.True))
	resp := method(t, client, "exchange", args(newRequestT(t, "GET", srv.URL, kw("headers", dict(str("X-Mine"), str("1"))))))
	got := rec.last(t)
	if got.proto != "HTTP/2.0" || get(t, resp, "version") != str("2") {
		t.Fatalf("proto = %s, version %v", got.proto, get(t, resp, "version"))
	}
	sameNames(t, "exchange over HTTP/2", got.header, "X-Mine")
	method(t, client, "get", args(str(srv.URL)))
	sameNames(t, "get over HTTP/2", rec.last(t).header, "User-Agent", "Accept-Encoding")
}

func TestVersion(t *testing.T) {
	srv := httptest.NewServer(&recorder{})
	defer srv.Close()
	if v := get(t, call(t, member(t, "get"), args(str(srv.URL))), "version"); v != str("1.1") {
		t.Fatalf("version = %v", v)
	}
}

// TestRejectConnect: CONNECT has no two-way stream to give.
func TestRejectConnect(t *testing.T) {
	req := newRequestT(t, "CONNECT", "http://proxy.test:3128")
	_, err := callErr(t, member(t, "send"), args(req))
	wantErr(t, err, object.ValueError, "send() cannot send a CONNECT request")
	_, err = callErr(t, member(t, "exchange"), args(req))
	wantErr(t, err, object.ValueError, "exchange() cannot send a CONNECT request")
	_, err = callErr(t, member(t, "request"), args(str("CONNECT"), str("http://proxy.test:3128")))
	wantErr(t, err, object.ValueError, "request() cannot send a CONNECT request")
}

// TestFramingFieldsAreOwned: the fields that describe one connection are
// computed, never taken from headers=; Host names the host.
func TestFramingFieldsAreOwned(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec)
	defer srv.Close()
	for _, fn := range []string{"exchange", "send"} {
		req := newRequestT(t, "POST", srv.URL, kw("body", str("hello world")), kw("headers", dict(
			str("Content-Length"), str("5"),
			str("Transfer-Encoding"), str("chunked"),
			str("Connection"), str("close"),
			str("Keep-Alive"), str("timeout=5"),
			str("Proxy-Connection"), str("keep-alive"),
			str("TE"), str("trailers"),
			str("Upgrade"), str("websocket"),
			str("Host"), str("api.example.test"),
			str("X-Kept"), str("1"),
		)))
		call(t, member(t, fn), args(req))
		got := rec.last(t)
		if got.body != "hello world" || got.contentLength != 11 || got.chunked {
			t.Fatalf("%s: body %q, length %d, chunked %v", fn, got.body, got.contentLength, got.chunked)
		}
		for _, name := range []string{"Connection", "Keep-Alive", "Proxy-Connection", "Te", "Upgrade", "Transfer-Encoding"} {
			if _, ok := got.header[name]; ok {
				t.Errorf("%s: %s reached the server", fn, name)
			}
		}
		if got.host != "api.example.test" || got.header.Get("X-Kept") != "1" {
			t.Fatalf("%s: host %q, headers %v", fn, got.host, got.header)
		}
	}
	// A received response's headers pass unchanged to a new message.
	first := call(t, member(t, "get"), args(str(srv.URL)))
	again := newRequestT(t, "POST", srv.URL, kw("headers", get(t, first, "headers")), kw("body", str("x")))
	call(t, member(t, "send"), args(again))
	got := rec.last(t)
	if got.header.Get("Content-Type") != "text/plain; charset=utf-8" || got.header.Get("Date") == "" || got.contentLength != 1 {
		t.Fatalf("forwarded headers: %v", got.header)
	}
}

// TestInterimResponses: a 1xx before the final response is skipped, and a
// request with Expect: 100-continue waits up to a second before its body.
func TestInterimResponses(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Header().Set("Link", "</style.css>; rel=preload")
		w.WriteHeader(stdhttp.StatusEarlyHints)
		io.WriteString(w, "final")
	}))
	defer srv.Close()
	for _, fn := range []string{"exchange", "send"} {
		resp := call(t, member(t, fn), args(newRequestT(t, "GET", srv.URL)))
		if get(t, resp, "status") != object.Integer(200) || text(t, resp) != "final" {
			t.Fatalf("%s: 103 was not skipped", fn)
		}
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	waited := make(chan time.Duration, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		br := bufio.NewReader(conn)
		req, err := stdhttp.ReadRequest(br)
		if err != nil {
			return
		}
		start := time.Now()
		data, _ := io.ReadAll(io.LimitReader(req.Body, req.ContentLength))
		waited <- time.Since(start)
		resp := "HTTP/1.1 200 OK\r\nContent-Length: " + itoa(len(data)) + "\r\n\r\n" + string(data)
		io.WriteString(conn, resp)
	}()
	req := newRequestT(t, "PUT", "http://"+ln.Addr().String(), kw("body", str("payload")), kw("headers", dict(str("Expect"), str("100-continue"))))
	resp := call(t, member(t, "exchange"), args(req))
	if text(t, resp) != "payload" {
		t.Fatal("body after the wait")
	}
	if d := <-waited; d < 800*time.Millisecond {
		t.Fatalf("the body came after %v, without waiting for a 100", d)
	}
}

// TestDeadReusedConnection: a request that fails on a reused connection
// before any response is sent once more on a new connection when it is
// idempotent (GET, or Idempotency-Key) and its body can be sent again, and
// not otherwise. This is the only repetition the library makes.
func TestDeadReusedConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	// Each connection answers its first request and then reads the next
	// one and drops the connection without an answer.
	arrivals := make(chan string, 16)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				br := bufio.NewReader(conn)
				for i := 0; ; i++ {
					req, err := stdhttp.ReadRequest(br)
					if err != nil {
						return
					}
					arrivals <- req.Method + " " + req.URL.Path
					if i == 1 {
						return
					}
					io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
				}
			}(conn)
		}
	}()
	base := "http://" + ln.Addr().String()
	drain := func() []string {
		var got []string
		for {
			select {
			case a := <-arrivals:
				got = append(got, a)
			case <-time.After(200 * time.Millisecond):
				return got
			}
		}
	}
	send := func(req object.Object) (object.Object, error) {
		return callErr(t, member(t, "send"), args(req))
	}
	warm := func() {
		resp, err := send(newRequestT(t, "GET", base+"/warm"))
		if err != nil {
			t.Fatal(err)
		}
		text(t, resp) // read to the end, which returns the connection to the pool
	}
	warm()
	resp, err := send(newRequestT(t, "GET", base+"/again"))
	if err != nil || text(t, resp) != "ok" {
		t.Fatalf("GET on a dead reused connection = %v", err)
	}
	if got := drain(); strings.Join(got, ",") != "GET /warm,GET /again,GET /again" {
		t.Fatalf("arrivals = %v", got)
	}
	// A POST is not repeated: the failure is reported. (The connection
	// the retry opened has answered one request, so it drops the next.)
	_, err = send(newRequestT(t, "POST", base+"/once", kw("body", str("x"))))
	wantErrPrefix(t, err, object.NetworkError, "send() cannot reach 127.0.0.1:")
	if got := drain(); strings.Join(got, ",") != "POST /once" {
		t.Fatalf("arrivals = %v", got)
	}
	// With an Idempotency-Key and bytes, it is.
	warm()
	resp, err = send(newRequestT(t, "POST", base+"/keyed", kw("body", str("x")), kw("headers", dict(str("Idempotency-Key"), str("k1")))))
	if err != nil || text(t, resp) != "ok" {
		t.Fatalf("keyed POST = %v", err)
	}
	if got := drain(); strings.Join(got, ",") != "GET /warm,POST /keyed,POST /keyed" {
		t.Fatalf("arrivals = %v", got)
	}
}

// TestConnectionsShared: connections are kept and reused across module
// calls and Clients with the same insecure setting, and not between the
// two settings.
func TestConnectionsShared(t *testing.T) {
	srv, conns := countedServer(&recorder{})
	defer srv.Close()
	for i := 0; i < 3; i++ {
		text(t, call(t, member(t, "get"), args(str(srv.URL))))
		text(t, method(t, newClientT(t), "get", args(str(srv.URL))))
		text(t, call(t, member(t, "exchange"), args(newRequestT(t, "GET", srv.URL))))
	}
	if n := conns.n.Load(); n != 1 {
		t.Fatalf("%d connections for nine requests", n)
	}
	text(t, method(t, newClientT(t, kw("insecure", object.True)), "get", args(str(srv.URL))))
	if n := conns.n.Load(); n != 2 {
		t.Fatalf("the insecure Client reused a verifying connection (%d connections)", n)
	}
}
