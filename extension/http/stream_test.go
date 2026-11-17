package http

import (
	"fmt"
	"io"
	"net"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func seconds64(f float64) object.Object { return object.Float(f) }

func TestTimeouts(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		flusher := w.(stdhttp.Flusher)
		switch r.URL.Path {
		case "/slow-head":
			select {
			case <-release:
			case <-time.After(5 * time.Second):
			}
			io.WriteString(w, "late")
		case "/half-second":
			time.Sleep(500 * time.Millisecond)
			io.WriteString(w, "late")
		case "/trickle":
			// Twelve pieces 50ms apart: 600ms in all, each gap well under
			// the 300ms limit.
			for i := 0; i < 12; i++ {
				fmt.Fprintf(w, "%d\n", i)
				flusher.Flush()
				time.Sleep(50 * time.Millisecond)
			}
		case "/stall":
			w.Header().Set("Content-Length", "100")
			io.WriteString(w, "half")
			flusher.Flush()
			select {
			case <-release:
			case <-time.After(5 * time.Second):
			}
		case "/error-stall":
			w.WriteHeader(500)
			io.WriteString(w, "partial")
			flusher.Flush()
			select {
			case <-release:
			case <-time.After(5 * time.Second):
			}
		}
	}))
	defer srv.Close()
	defer close(release) // before Close, which waits for the handlers
	host := strings.TrimPrefix(srv.URL, "http://")
	limit := kw("timeout", seconds64(0.3))

	// The head is late: TimeoutError from the call, for send and exchange.
	_, err := callErr(t, member(t, "get"), args(str(srv.URL+"/slow-head")), limit)
	wantErr(t, err, object.TimeoutError, "get() timed out after 0.3 seconds")
	_, err = callErr(t, member(t, "exchange"), args(newRequestT(t, "GET", srv.URL+"/slow-head")), limit)
	wantErr(t, err, object.TimeoutError, "exchange() timed out after 0.3 seconds")
	_, err = methodErr(t, newClientT(t, limit), "send", args(newRequestT(t, "GET", srv.URL+"/slow-head")))
	wantErr(t, err, object.TimeoutError, "send() timed out after 0.3 seconds")
	// The Client's timeout is replaced by a per-call one, nil included.
	slowClient := newClientT(t, limit)
	if text(t, method(t, slowClient, "get", args(str(srv.URL+"/half-second")), kw("timeout", object.Nil))) != "late" {
		t.Fatal("timeout=nil")
	}
	if !errorsIsKind(err, object.IOError) {
		t.Fatal("TimeoutError is not an IOError")
	}

	// A body that keeps arriving is never cut off.
	resp := call(t, member(t, "get"), args(str(srv.URL+"/trickle")), limit)
	if got := text(t, resp); strings.Count(got, "\n") != 12 {
		t.Fatalf("trickled body = %q", got)
	}
	// A body that stops: TimeoutError from the read, naming no function.
	resp = call(t, member(t, "get"), args(str(srv.URL+"/stall")), limit)
	if got := method(t, body(t, resp), "read", args(object.Integer(4))); string(got.(object.Bytes)) != "half" {
		t.Fatalf("first part = %v", got)
	}
	_, err = methodErr(t, body(t, resp), "read", nil)
	wantErr(t, err, object.TimeoutError, "timed out after 0.3 seconds waiting for the response body from "+host)
	resp = call(t, member(t, "exchange"), args(newRequestT(t, "GET", srv.URL+"/stall")), limit)
	_, err = object.Collect(body(t, resp))
	wantErr(t, err, object.TimeoutError, "timed out after 0.3 seconds waiting for the response body from "+host)
	// Reading an error body is part of the call.
	start := time.Now()
	_, err = callErr(t, member(t, "get"), args(str(srv.URL+"/error-stall")), limit)
	wantErr(t, err, object.TimeoutError, "get() timed out after 0.3 seconds")
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("error body read took %v", d)
	}
	// Bad timeouts.
	_, err = callErr(t, member(t, "get"), args(str(srv.URL)), kw("timeout", object.Integer(-1)))
	wantErr(t, err, object.ValueError, "get() argument 'timeout' must be non-negative, got -1")
	_, err = callErr(t, clientType.Function, nil, kw("timeout", str("30")))
	wantErr(t, err, object.TypeError, "Client() argument 'timeout' must be Int, Float or Nil, got String")
}

func TestNetworkErrors(t *testing.T) {
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Header().Set("Content-Length", "100")
		io.WriteString(w, "cut")
		w.(stdhttp.Flusher).Flush()
		conn, _, _ := w.(stdhttp.Hijacker).Hijack()
		conn.Close()
	}))
	url := srv.URL
	host := strings.TrimPrefix(url, "http://")
	resp := call(t, member(t, "get"), args(str(url)))
	_, err := methodErr(t, body(t, resp), "read", nil)
	wantErrPrefix(t, err, object.NetworkError, "cannot read the response body from "+host+": ")
	srv.Close()
	_, err = callErr(t, member(t, "get"), args(str(url)))
	wantErrPrefix(t, err, object.NetworkError, "get() cannot reach "+host+": ")
	_, err = callErr(t, member(t, "request"), args(str("POST"), str(url)))
	wantErrPrefix(t, err, object.NetworkError, "request() cannot reach "+host+": ")
}

// TestRelease: a body read to its end, a closed body and a StatusError each
// leave the connection reusable.
func TestRelease(t *testing.T) {
	srv, conns := countedServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path == "/fail" {
			stdhttp.Error(w, strings.Repeat("x", 10000), 500)
			return
		}
		io.WriteString(w, strings.Repeat("line\n", 1000))
	}))
	defer srv.Close()
	client := newClientT(t)
	get1 := func(path string) (object.Object, error) {
		return methodErr(t, client, "get", args(str(srv.URL+path)))
	}
	r, _ := get1("/")
	method(t, body(t, r), "read", nil)
	if get(t, body(t, r), "closed") != object.False {
		t.Fatal("closed is true after reading to the end")
	}
	r, _ = get1("/")
	method(t, body(t, r), "close", nil)
	if get(t, body(t, r), "closed") != object.True {
		t.Fatal("closed after close()")
	}
	_, err := get1("/fail")
	wantErrPrefix(t, err, StatusError, "get() ")
	r, _ = get1("/")
	text(t, r)
	if n := conns.n.Load(); n > 2 {
		// Closing early may cost the connection whose body was cut short
		// (Go closes it rather than drain a large rest); everything else
		// reuses one.
		t.Fatalf("%d connections", n)
	}
	before := conns.n.Load()
	for i := 0; i < 3; i++ {
		_, err := get1("/fail")
		wantErrPrefix(t, err, StatusError, "get() ")
		r, _ = get1("/")
		text(t, r)
	}
	if conns.n.Load() != before {
		t.Fatalf("StatusError or a full read did not release the connection: %d -> %d", before, conns.n.Load())
	}
}

// TestDroppedBodyReleased: a Response dropped unread releases its
// connection when it is collected.
func TestDroppedBodyReleased(t *testing.T) {
	var active atomic.Int32
	srv := httptest.NewUnstartedServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		io.WriteString(w, strings.Repeat("x", 64<<20))
	}))
	srv.Config.ConnState = func(_ net.Conn, state stdhttp.ConnState) {
		switch state {
		case stdhttp.StateActive:
			active.Add(1)
		case stdhttp.StateIdle, stdhttp.StateClosed:
			active.Add(-1)
		}
	}
	srv.Start()
	defer srv.Close()
	func() {
		call(t, member(t, "get"), args(str(srv.URL)))
	}()
	deadline := time.Now().Add(5 * time.Second)
	for active.Load() != 0 {
		if time.Now().After(deadline) {
			srv.CloseClientConnections()
			t.Fatal("an unread, dropped Response kept its connection")
		}
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
	}
}

// TestLines: for line in response.body as it arrives; break, then read()
// continues where the loop stopped; close() releases.
func TestLines(t *testing.T) {
	gate := make(chan struct{})
	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		fmt.Fprint(w, "data: 1\r\n")
		w.(stdhttp.Flusher).Flush()
		if r.URL.Path == "/gated" {
			<-gate
		}
		fmt.Fprint(w, "data: 2\n\nlast")
	}))
	defer srv.Close()
	resp := call(t, member(t, "get"), args(str(srv.URL)), kw("timeout", object.Nil))
	lines, err := object.Collect(body(t, resp))
	if err != nil || !modtest.Equal(list(lines...), list(str("data: 1"), str("data: 2"), str(""), str("last"))) {
		t.Fatalf("lines = %v %v", lines, err)
	}
	// The first line arrives before the server sends the rest.
	resp = call(t, member(t, "get"), args(str(srv.URL+"/gated")))
	it, err := object.Iterate(body(t, resp))
	if err != nil {
		t.Fatal(err)
	}
	first, ok, err := it.Next()
	if !ok || err != nil || first != str("data: 1") {
		t.Fatalf("first line = %v %v %v", first, ok, err)
	}
	it.Close() // break
	close(gate)
	if rest := method(t, body(t, resp), "read", nil); string(rest.(object.Bytes)) != "data: 2\n\nlast" {
		t.Fatalf("read after break = %q", rest)
	}
	// Lines that are not UTF-8: the File's ValueError.
	bad := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Write([]byte("ok\n\xff\n"))
	}))
	defer bad.Close()
	resp = call(t, member(t, "get"), args(str(bad.URL)))
	_, err = object.Collect(body(t, resp))
	wantErr(t, err, object.ValueError, "line 2 of body is not valid UTF-8")
	_, err = methodErr(t, call(t, member(t, "get"), args(str(bad.URL))), "text", nil)
	wantErr(t, err, object.ValueError, "text() body is not valid UTF-8")
}

// TestProxyFromEnvironment runs in a child process, because Go reads the
// proxy variables once per process.
func TestProxyFromEnvironment(t *testing.T) {
	if os.Getenv("GOBLIN_HTTP_PROXY_CHILD") == "1" {
		resp := call(t, member(t, "get"), args(str("http://goblin.invalid/through?q=1")))
		fmt.Println("PROXIED:", text(t, resp))
		return
	}
	proxy := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		fmt.Fprintf(w, "%s %s", r.Method, r.RequestURI)
	}))
	defer proxy.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestProxyFromEnvironment$", "-test.v")
	cmd.Env = append(os.Environ(), "GOBLIN_HTTP_PROXY_CHILD=1", "HTTP_PROXY="+proxy.URL, "http_proxy="+proxy.URL, "NO_PROXY=", "no_proxy=")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "PROXIED: GET http://goblin.invalid/through?q=1") {
		t.Fatalf("child: %v\n%s", err, out)
	}
}
