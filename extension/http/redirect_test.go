package http

import (
	"fmt"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/object"
)

func redirectServer(t *testing.T) (*httptest.Server, *recorder) {
	rec := &recorder{}
	rec.next = func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/chain/"):
			// /chain/N redirects N times before answering.
			n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/chain/"))
			if n == 0 {
				io.WriteString(w, "end of chain")
				return
			}
			stdhttp.Redirect(w, r, fmt.Sprintf("/chain/%d", n-1), stdhttp.StatusFound)
		case r.URL.Path == "/see-other":
			stdhttp.Redirect(w, r, "/landed", stdhttp.StatusSeeOther)
		case r.URL.Path == "/temporary":
			stdhttp.Redirect(w, r, "/landed", stdhttp.StatusTemporaryRedirect)
		case r.URL.Path == "/permanent":
			stdhttp.Redirect(w, r, "/landed", stdhttp.StatusPermanentRedirect)
		case r.URL.Path == "/no-location":
			w.WriteHeader(stdhttp.StatusFound)
		case r.URL.Path == "/landed":
			fmt.Fprintf(w, "%s %s", r.Method, rec.last(t).body)
		case r.URL.Path == "/away":
			stdhttp.Redirect(w, r, r.URL.Query().Get("to"), stdhttp.StatusFound)
		default:
			io.WriteString(w, "ok")
		}
	}
	return httptest.NewServer(rec), rec
}

func TestRedirects(t *testing.T) {
	srv, rec := redirectServer(t)
	defer srv.Close()

	// The default limit is 10, and Response.url says where the chain ended.
	resp := call(t, member(t, "get"), args(str(srv.URL+"/chain/10")))
	if text(t, resp) != "end of chain" || get(t, resp, "url").(fmt.Stringer).String() != srv.URL+"/chain/0" {
		t.Fatal("a chain of 10")
	}
	_, err := callErr(t, member(t, "get"), args(str(srv.URL+"/chain/11")))
	wantErr(t, err, StatusError, "get() "+srv.URL+"/chain/11 was redirected more than 10 times")

	// redirects=2 on a chain of 3: StatusError with the last 302, or that
	// 302 with check=false.
	two := newClientT(t, kw("redirects", object.Integer(2)))
	_, err = methodErr(t, two, "get", args(str(srv.URL+"/chain/3")))
	wantErr(t, err, StatusError, "get() "+srv.URL+"/chain/3 was redirected more than 2 times")
	last := field(t, err, "response")
	if field(t, err, "status") != object.Integer(302) || get(t, last, "url").(fmt.Stringer).String() != srv.URL+"/chain/1" ||
		method(t, get(t, last, "headers"), "get", args(str("location"))) != str("/chain/0") {
		t.Fatalf("limit data: %v", last)
	}
	resp = method(t, two, "get", args(str(srv.URL+"/chain/3")), kw("check", object.False))
	if get(t, resp, "status") != object.Integer(302) {
		t.Fatal("check=false at the limit")
	}
	if text(t, method(t, two, "get", args(str(srv.URL+"/chain/2")))) != "end of chain" {
		t.Fatal("two redirects with redirects=2")
	}

	// redirects=0 returns every 3xx, and check does not raise for it.
	zero := newClientT(t, kw("redirects", object.Integer(0)))
	resp = method(t, zero, "get", args(str(srv.URL+"/chain/1")))
	if get(t, resp, "status") != object.Integer(302) {
		t.Fatal("redirects=0 followed")
	}
	resp = method(t, zero, "request", args(str("PUT"), str(srv.URL+"/temporary")), kw("body", filestream.NewReader("pipe", strings.NewReader("x"), nil)))
	if get(t, resp, "status") != object.Integer(307) {
		t.Fatal("redirects=0 with a stream")
	}
	// A 3xx without a Location is an answer.
	if get(t, call(t, member(t, "get"), args(str(srv.URL+"/no-location"))), "status") != object.Integer(302) {
		t.Fatal("302 without Location")
	}

	// 303 after a POST arrives as a GET without the body's headers.
	resp = call(t, member(t, "request"), args(str("POST"), str(srv.URL+"/see-other")), kw("body", str("payload")), kw("headers", dict(
		str("Content-Type"), str("text/plain"), str("Content-Encoding"), str("identity"),
		str("Content-Language"), str("en"), str("Content-Location"), str("/x"), str("X-Kept"), str("1"))))
	landed := rec.last(t)
	if text(t, resp) != "GET " || landed.method != "GET" || landed.contentLength != 0 {
		t.Fatalf("303: %v", landed)
	}
	for _, name := range []string{"Content-Type", "Content-Encoding", "Content-Language", "Content-Location"} {
		if _, ok := landed.header[name]; ok {
			t.Errorf("%s sent after a 303", name)
		}
	}
	if landed.header.Get("X-Kept") != "1" {
		t.Fatal("other headers dropped after 303")
	}
	// A HEAD stays a HEAD.
	call(t, member(t, "request"), args(str("HEAD"), str(srv.URL+"/see-other")))
	if rec.last(t).method != "HEAD" {
		t.Fatal("HEAD became GET")
	}

	// 307 and 308 with bytes: the same method and body.
	for _, path := range []string{"/temporary", "/permanent"} {
		resp = call(t, member(t, "request"), args(str("PUT"), str(srv.URL+path)), kw("body", str("again")), kw("headers", dict(str("Content-Type"), str("text/plain"))))
		landed = rec.last(t)
		if text(t, resp) != "PUT again" || landed.header.Get("Content-Type") != "text/plain" {
			t.Fatalf("%s: %v", path, landed)
		}
	}

	// 307 to a stream body: StatusError with check=true, the 307 with false.
	for _, path := range []string{"/temporary", "/permanent"} {
		pipe := filestream.NewReader("pipe", strings.NewReader("streamed"), nil)
		_, err = callErr(t, member(t, "request"), args(str("PUT"), str(srv.URL+path)), kw("body", pipe))
		status := map[string]int{"/temporary": 307, "/permanent": 308}[path]
		wantErr(t, err, StatusError, fmt.Sprintf("request() %s%s answered %d and the body is a stream that cannot be sent again", srv.URL, path, status))
		if field(t, err, "status") != object.Integer(status) {
			t.Fatal("status data")
		}
		pipe = filestream.NewReader("pipe", strings.NewReader("streamed"), nil)
		resp = call(t, member(t, "request"), args(str("PUT"), str(srv.URL+path)), kw("body", pipe), kw("check", object.False))
		if get(t, resp, "status") != object.Integer(status) {
			t.Fatal("check=false with a stream")
		}
	}

	// Credentials are not sent to another host; they are to the same one.
	other := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	secret := dict(str("Authorization"), str("Bearer s"), str("Cookie"), str("c=1"), str("Proxy-Authorization"), str("Basic p"), str("X-Kept"), str("1"))
	call(t, member(t, "get"), args(str(srv.URL+"/away?to="+other+"/landed")), kw("headers", secret))
	landed = rec.last(t)
	if landed.host != strings.TrimPrefix(other, "http://") || landed.header.Get("X-Kept") != "1" {
		t.Fatalf("cross-host hop: %v", landed)
	}
	for _, name := range []string{"Authorization", "Cookie", "Proxy-Authorization"} {
		if _, ok := landed.header[name]; ok {
			t.Errorf("%s sent to another host", name)
		}
	}
	call(t, member(t, "get"), args(str(srv.URL+"/away?to=/landed")), kw("headers", secret))
	if rec.last(t).header.Get("Authorization") != "Bearer s" {
		t.Fatal("Authorization dropped on the same host")
	}

	// redirects is an Int, 0 or more.
	_, err = callErr(t, clientType.Function, nil, kw("redirects", object.Integer(-1)))
	wantErr(t, err, object.ValueError, "Client() argument 'redirects' must be non-negative, got -1")
	_, err = callErr(t, clientType.Function, nil, kw("redirects", object.False))
	wantErr(t, err, object.TypeError, "Client() argument 'redirects' must be Int, got Bool")
}
