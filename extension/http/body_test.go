package http

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/object"
)

// lying is a body that claims a length it does not have.
type lying struct {
	io.Reader
	n int64
}

func (l lying) Remaining() (int64, bool) { return l.n, true }

func TestBodySending(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec)
	defer srv.Close()
	send := func(b object.Object) seen {
		t.Helper()
		call(t, member(t, "request"), args(str("PUT"), str(srv.URL)), kw("body", b))
		return rec.last(t)
	}

	// Declared lengths.
	if got := send(str("héllo")); got.contentLength != 6 || got.chunked || got.body != "héllo" {
		t.Fatalf("String: %+v", got)
	}
	if got := send(object.Bytes{0, 1, 2}); got.contentLength != 3 || got.body != "\x00\x01\x02" {
		t.Fatalf("Bytes: %+v", got)
	}
	if got := send(object.Nil); got.contentLength != 0 || got.body != "" {
		t.Fatalf("nil: %+v", got)
	}
	path := filepath.Join(t.TempDir(), "backup.tar.gz")
	os.WriteFile(path, []byte("0123456789"), 0o644)
	f := goblinFile(t, path)
	method(t, f, "read", args(object.Integer(3)))
	if got := send(f); got.contentLength != 7 || got.body != "3456789" {
		t.Fatalf("File from its position: %+v", got)
	}
	if get(t, f, "closed") != object.False {
		t.Fatal("the body's file was closed")
	}
	// A File whose lines have been read sends what line reading buffered.
	os.WriteFile(path, []byte("first\nsecond\n"), 0o644)
	g2 := goblinFile(t, path)
	defer g2.Close(object.CallArgs{})
	method(t, g2, "read_line", nil)
	if got := send(g2); got.body != "second\n" || got.contentLength != 7 {
		t.Fatalf("File after read_line: %+v", got)
	}
	// Unknown length: chunked.
	pipe := filestream.NewReader("pipe", strings.NewReader("piped"), nil)
	if got := send(pipe); !got.chunked || got.contentLength != -1 || got.body != "piped" {
		t.Fatalf("pipe: %+v", got)
	}
	closed := false
	if got := send(readable(strings.NewReader("ducked"), &closed)); !got.chunked || got.body != "ducked" || closed {
		t.Fatalf("readable: %+v (closed %v)", got, closed)
	}

	// A bytes Request is sent whole twice, also after its body was read.
	req := newRequestT(t, "PUT", srv.URL, kw("body", str("again")))
	call(t, member(t, "send"), args(req))
	method(t, body(t, req), "read", nil)
	call(t, member(t, "exchange"), args(req))
	if got := rec.last(t); got.body != "again" || got.contentLength != 5 {
		t.Fatalf("second send: %+v", got)
	}
	// A stream Request is good for one send or exchange.
	streamReq := newRequestT(t, "PUT", srv.URL, kw("body", filestream.NewReader("pipe", strings.NewReader("once"), nil)))
	call(t, member(t, "exchange"), args(streamReq))
	for _, fn := range []string{"send", "exchange"} {
		_, err := callErr(t, member(t, fn), args(streamReq))
		wantErr(t, err, object.ValueError, fn+"() the body of this request is a stream that was already sent; build the request again")
	}

	// A readable that raises: its error, unchanged.
	raising := &object.Module{Name: "broken", Members: map[string]object.Object{
		"read": &object.Function{Name: "read", Fn: func(object.CallArgs) (object.Object, error) {
			return nil, object.NewKindError(object.KeyError, "the source broke").With("path", str("src"))
		}},
	}}
	_, err := callErr(t, member(t, "request"), args(str("POST"), str(srv.URL)), kw("body", raising))
	if !errorsIsKind(err, object.KeyError) || !strings.Contains(err.Error(), "the source broke") || field(t, err, "path") != str("src") {
		t.Fatalf("raising body = %v", err)
	}
	// A closed File raises its ValueError.
	method(t, f, "close", nil)
	_, err = callErr(t, member(t, "request"), args(str("POST"), str(srv.URL)), kw("body", f))
	if !errorsIsKind(err, object.ValueError) || !strings.Contains(err.Error(), "closed file") {
		t.Fatalf("closed file body = %v", err)
	}
	// A declared length that does not match the data is a NetworkError.
	for _, l := range []lying{{strings.NewReader("short"), 100}, {strings.NewReader("much longer"), 2}} {
		_, err := callErr(t, member(t, "request"), args(str("POST"), str(srv.URL)), kw("body", filestream.NewReader("liar", l, nil)))
		wantErrPrefix(t, err, object.NetworkError, "request() the body changed size while it was sent to 127.0.0.1:")
	}
}

func errorsIsKind(err error, kind *object.Error) bool { return errors.Is(err, kind) }

// TestForwarding: a received body and its headers make a new request; the
// length the server declared is kept, and a decoded body is chunked.
func TestForwarding(t *testing.T) {
	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path == "/gzip" && r.Header.Get("Accept-Encoding") == "gzip" {
			w.Header().Set("Content-Encoding", "gzip")
			zw := gzip.NewWriter(w)
			io.WriteString(zw, "unzipped")
			zw.Close()
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		io.WriteString(w, "forwarded bytes")
	}))
	defer upstream.Close()
	rec := &recorder{}
	target := httptest.NewServer(rec)
	defer target.Close()

	got := call(t, member(t, "get"), args(str(upstream.URL)))
	passed := call(t, requestType.Function, args(str("PUT"), str(target.URL)), kw("headers", get(t, got, "headers")), kw("body", body(t, got)))
	call(t, member(t, "exchange"), args(passed))
	seen := rec.last(t)
	if seen.body != "forwarded bytes" || seen.contentLength != 15 || seen.chunked || seen.header.Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("forwarded: %+v", seen)
	}
	got = call(t, member(t, "get"), args(str(upstream.URL+"/gzip")))
	call(t, member(t, "request"), args(str("PUT"), str(target.URL)), kw("body", body(t, got)))
	if seen = rec.last(t); seen.body != "unzipped" || !seen.chunked {
		t.Fatalf("decoded body forwarded: %+v", seen)
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

func lineOf(body, name string) string {
	for _, line := range strings.Split(body, "\n") {
		if v, ok := strings.CutPrefix(line, name+"="); ok {
			return v
		}
	}
	return "<missing " + name + ">"
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
	form, contentType := parts[0], string(parts[1].(object.String))
	if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") || fmt.Sprint(form) != "<File multipart>" || get(t, form, "path") != object.Nil {
		t.Fatalf("multipart() = %v %q", form, contentType)
	}
	resp := call(t, member(t, "request"), args(str("POST"), str(srv.URL)), kw("headers", dict(str("Content-Type"), str(contentType))), kw("body", form))
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
	// The library closed nothing: the caller's file is open.
	if get(t, f, "closed") != object.False {
		t.Fatal("the caller's file was closed")
	}
	// The body is good for one request.
	_, err := callErr(t, member(t, "request"), args(str("POST"), str(srv.URL)), kw("body", form))
	wantErr(t, err, object.ValueError, "request() this multipart body was already sent; build it again")
	_, err = callErr(t, member(t, "send"), args(newRequestT(t, "POST", srv.URL, kw("body", form))))
	wantErr(t, err, object.ValueError, "send() this multipart body was already sent; build it again")

	// A part whose length is unknown makes the body chunked.
	unknown := filestream.NewReader("pipe", strings.NewReader("piped"), nil)
	result = call(t, member(t, "multipart"), nil, kw("files", dict(str("avatar"), dict(str("data"), unknown, str("name"), str("p.txt")))))
	parts = result.(*object.List).Elements
	got = text(t, call(t, member(t, "request"), args(str("POST"), str(srv.URL)), kw("headers", dict(str("Content-Type"), parts[1])), kw("body", parts[0])))
	if lineOf(got, "transfer-encoding") != "chunked" || !strings.Contains(got, "file avatar p.txt application/octet-stream piped") {
		t.Fatalf("chunked multipart: %q", got)
	}

	// An empty form is valid.
	parts = call(t, member(t, "multipart"), nil).(*object.List).Elements
	resp = call(t, member(t, "request"), args(str("POST"), str(srv.URL)), kw("headers", dict(str("Content-Type"), parts[1])), kw("body", parts[0]))
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
		if err == nil || !errorsIsKind(err, c.kind) || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("multipart(files=%v) = %v, want %q", c.files, err, c.msg)
		}
	}
	if _, err := callErr(t, member(t, "multipart"), nil, kw("fields", dict(str("a"), dict()))); !errorsIsKind(err, object.TypeError) {
		t.Errorf("fields with a Dict value = %v", err)
	}
}

func TestInsecure(t *testing.T) {
	srv := httptest.NewUnstartedServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		stdhttp.SetCookie(w, &stdhttp.Cookie{Name: "seen", Value: "1", Path: "/", Secure: true})
		if r.URL.Path == "/hop" {
			stdhttp.Redirect(w, r, "/end", stdhttp.StatusFound)
			return
		}
		fmt.Fprintf(w, "proto=%s path=%s", r.Proto, r.URL.Path)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	_, err := callErr(t, member(t, "get"), args(str(srv.URL)))
	wantErrPrefix(t, err, object.NetworkError, "get() cannot reach 127.0.0.1:")
	_, err = methodErr(t, newClientT(t), "get", args(str(srv.URL)))
	wantErrPrefix(t, err, object.NetworkError, "get() cannot reach")
	_, err = callErr(t, member(t, "exchange"), args(newRequestT(t, "GET", srv.URL)))
	wantErrPrefix(t, err, object.NetworkError, "exchange() cannot reach")

	client := newClientT(t, kw("insecure", object.True))
	// Redirects included.
	if got := text(t, method(t, client, "get", args(str(srv.URL+"/hop")))); got != "proto=HTTP/2.0 path=/end" {
		t.Fatalf("insecure client = %q (the clone lost HTTP/2?)", got)
	}
	if got := text(t, method(t, client, "exchange", args(newRequestT(t, "GET", srv.URL+"/x")))); got != "proto=HTTP/2.0 path=/x" {
		t.Fatalf("insecure exchange = %q", got)
	}
	for _, insecure := range []bool{false, true} {
		tr := transport(insecure)
		if tr.Proxy == nil || !tr.ForceAttemptHTTP2 || !tr.DisableCompression || tr.ExpectContinueTimeout == 0 {
			t.Fatalf("transport(%v) lost a setting of Go's default", insecure)
		}
	}
	if transport(false).TLSClientConfig != nil && transport(false).TLSClientConfig.InsecureSkipVerify {
		t.Fatal("the verifying transport skips verification")
	}
	// A Secure cookie for an https URL is kept for every path; loopback
	// counts as secure, as in browsers.
	if got := method(t, client, "cookies", args(str(srv.URL+"/other/path"))).(*object.Dict); got.Size() != 1 {
		t.Fatalf("cookies(https) = %v", got)
	}
	plain := strings.Replace(srv.URL, "https://", "http://", 1)
	if got := method(t, client, "cookies", args(str(plain))).(*object.Dict); got.Size() != 1 {
		t.Fatalf("cookies(http on loopback) = %v", got)
	}
}

// TestMultipartEncodesNames checks that a field name or file name cannot end
// its part header: CR, LF and the quote are percent-encoded, as browsers
// encode them, and Go's mime/multipart reads the names back.
func TestMultipartEncodesNames(t *testing.T) {
	result := call(t, member(t, "multipart"), nil,
		kw("fields", dict(str("a\"b"), str("v"))),
		kw("files", dict(str("f\r\n"), dict(str("data"), str("x"), str("name"), str("we\"ird\r\nname.txt")))))
	parts := result.(*object.List).Elements
	data := string(method(t, parts[0], "read", nil).(object.Bytes))
	for _, want := range []string{
		`Content-Disposition: form-data; name="a%22b"` + "\r\n",
		`Content-Disposition: form-data; name="f%0D%0A"; filename="we%22ird%0D%0Aname.txt"` + "\r\n",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("body %q lacks %q", data, want)
		}
	}
	if strings.Contains(data, "\\\"") || strings.Contains(data, "ird\r\n") {
		t.Fatalf("a name was written raw: %q", data)
	}
}
