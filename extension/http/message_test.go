package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/extension/internal/urlvalue"
	"github.com/aisk/goblin/object"
)

func TestRequestValue(t *testing.T) {
	src := dict(str("X-Id"), str("1"))
	r := newRequestT(t, "POST", "https://example.com/users?a=1", kw("headers", src), kw("body", str("hello")))
	if r.(*Request).String() != "<Request POST https://example.com/users?a=1>" {
		t.Fatalf("Show = %v", r)
	}
	if get(t, r, "method") != str("POST") {
		t.Fatal("method")
	}
	u := get(t, r, "url")
	if _, ok := u.(*urlvalue.URL); !ok || u.(*urlvalue.URL).String() != "https://example.com/users?a=1" {
		t.Fatalf("url = %v", u)
	}
	h := get(t, r, "headers")
	if h != get(t, r, "headers") {
		t.Fatal("headers is not the same object on every read")
	}
	src.Set(str("X-Late"), str("1"))
	if h.(*Headers).has("x-late") {
		t.Fatal("the Dict passed in is aliased")
	}
	b := body(t, r)
	if b != body(t, r) || b.String() != "<File body>" || get(t, b, "path") != object.Nil {
		t.Fatalf("body = %v", b)
	}
	if text(t, r) != "hello" || text(t, r) != "" {
		t.Fatal("text() twice")
	}
	if _, err := methodErr(t, b, "write", args(str("x"))); err == nil || !strings.Contains(err.Error(), "not open for writing") {
		t.Fatalf("body.write = %v", err)
	}
	if ok, _ := r.SetAttr("method", str("GET")); ok {
		t.Fatal("method is writable")
	}
	// type_of
	if r.(*Request).Constructor() != requestType.Function || get(t, Must(Execute()), "Request") != requestType.Function {
		t.Fatal("type_of(request) != http.Request")
	}

	// Body kinds.
	if data := method(t, body(t, newRequestT(t, "GET", "http://x.test")), "read", nil); !modtest.Equal(data, object.Bytes{}) {
		t.Fatalf("nil body reads %v", data)
	}
	if text(t, newRequestT(t, "PUT", "http://x.test", kw("body", object.Bytes("bytes")))) != "bytes" {
		t.Fatal("Bytes body")
	}
	path := filepath.Join(t.TempDir(), "f.txt")
	os.WriteFile(path, []byte("from a file"), 0o644)
	f := goblinFile(t, path)
	defer f.Close(object.CallArgs{})
	if body(t, newRequestT(t, "PUT", "http://x.test", kw("body", f))) != f {
		t.Fatal("a File body is not that File")
	}
	closed := false
	duck := newRequestT(t, "PUT", "http://x.test", kw("body", readable(strings.NewReader("ducked"), &closed)))
	db := body(t, duck)
	if db.String() != "<File body>" || text(t, duck) != "ducked" {
		t.Fatal("duck body")
	}
	method(t, db, "close", nil)
	if closed {
		t.Fatal("closing the body File closed the readable")
	}
	// text() on bytes that are not UTF-8.
	bad := newRequestT(t, "PUT", "http://x.test", kw("body", object.Bytes{0xff, 0xfe}))
	_, err := methodErr(t, bad, "text", nil)
	wantErr(t, err, object.ValueError, "text() body is not valid UTF-8")

	cases := []struct {
		args []object.Object
		kws  []object.KwArg
		kind *object.Error
		msg  string
	}{
		{args(str("GET"), str("ht!tp://x")), nil, object.ParseError, `Request() invalid URL "ht!tp://x": first path segment in URL cannot contain colon`},
		{args(str("GET"), str("users/7")), nil, object.ValueError, `Request() needs an absolute URL, got "users/7"`},
		{args(str("GET"), str("ftp://example.com/f")), nil, object.ValueError, `Request() URL scheme must be http or https, got "ftp"`},
		{args(str("GET"), str("https://ada:pw@example.com/")), nil, object.ValueError, "Request() URL must not contain a user name or password; send an Authorization header"},
		{args(str("GET"), str("https://ada@example.com/")), nil, object.ValueError, "Request() URL must not contain a user name or password; send an Authorization header"},
		{args(str("GET /"), str("http://x.test")), nil, object.ValueError, `Request() invalid method "GET /"`},
		{args(str(""), str("http://x.test")), nil, object.ValueError, `Request() invalid method ""`},
		{args(str("POST"), str("http://x.test")), []object.KwArg{kw("body", object.Integer(42))}, object.TypeError, "Request() argument 'body' must be String, Bytes, Nil or have a read(size) method, got Int"},
		{args(str("GET"), object.Integer(1)), nil, object.TypeError, "Request() argument 'url' must be String or URL, got Int"},
	}
	for _, c := range cases {
		_, err := callErr(t, requestType.Function, c.args, c.kws...)
		wantErr(t, err, c.kind, c.msg)
	}
	_, err = callErr(t, requestType.Function, args(str("GET"), str("ht!tp://x")))
	if field(t, err, "text") != str("ht!tp://x") {
		t.Fatal("ParseError text")
	}
	// A url.URL is accepted; a lower-case method is sent as written.
	parsed, _ := urlvalue.Parse("parse", "http://x.test/p")
	if get(t, call(t, requestType.Function, args(str("get"), parsed)), "method") != str("get") {
		t.Fatal("method as written")
	}
	// request() and get() raise the same errors under their own names.
	_, err = callErr(t, member(t, "get"), args(str("users/7")))
	wantErr(t, err, object.ValueError, `get() needs an absolute URL, got "users/7"`)
	_, err = callErr(t, member(t, "request"), args(str("GE T"), str("http://x.test")))
	wantErr(t, err, object.ValueError, `request() invalid method "GE T"`)
	_, err = callErr(t, member(t, "request"), args(str("POST"), str("http://x.test")), kw("body", object.Integer(1)))
	wantErr(t, err, object.TypeError, "request() argument 'body' must be String, Bytes, Nil or have a read(size) method, got Int")
	_, err = callErr(t, member(t, "get"), args(str("https://u:p@x.test/")))
	wantErr(t, err, object.ValueError, "get() URL must not contain a user name or password; send an Authorization header")
}

func Must(v object.Object, err error) object.Object {
	if err != nil {
		panic(err)
	}
	return v
}

func TestResponseValue(t *testing.T) {
	r := call(t, responseType.Function, args(object.Integer(201)), kw("headers", dict(str("Location"), str("/users/7"))), kw("body", str("made")))
	if r.(*Response).String() != "<Response 201>" || get(t, r, "status") != object.Integer(201) {
		t.Fatalf("Response = %v", r)
	}
	if get(t, r, "url") != object.Nil || get(t, r, "version") != object.Nil {
		t.Fatal("a built Response has a url or version")
	}
	if get(t, r, "headers") != get(t, r, "headers") || method(t, get(t, r, "headers"), "get", args(str("location"))) != str("/users/7") {
		t.Fatal("headers")
	}
	if body(t, r) != body(t, r) || text(t, r) != "made" || text(t, r) != "" {
		t.Fatal("body")
	}
	empty := call(t, responseType.Function, args(object.Integer(204)))
	if !modtest.Equal(method(t, body(t, empty), "read", nil), object.Bytes{}) {
		t.Fatal("empty body")
	}
	var f *filestream.File = filestream.NewReader("pipe", strings.NewReader("x"), nil)
	if body(t, call(t, responseType.Function, args(object.Integer(200)), kw("body", f))) != f {
		t.Fatal("a File body is not that File")
	}
	for _, status := range []int64{20, 99, 1000} {
		_, err := callErr(t, responseType.Function, args(object.Integer(status)))
		wantErrPrefix(t, err, object.ValueError, "Response() argument 'status' must be from 100 to 999, got ")
	}
	_, err := callErr(t, responseType.Function, args(object.Integer(200)), kw("body", object.Integer(1)))
	wantErr(t, err, object.TypeError, "Response() argument 'body' must be String, Bytes, Nil or have a read(size) method, got Int")
	_, err = callErr(t, responseType.Function, args(str("200")))
	wantErr(t, err, object.TypeError, "Response() argument 'status' must be Int, got String")
	// Messages compare by identity and are not hashable.
	other := call(t, responseType.Function, args(object.Integer(201)))
	if modtest.Equal(r, other) || !modtest.Equal(r, r) {
		t.Fatal("Response equality is not identity")
	}
	if _, err := object.Hash(r); err == nil {
		t.Fatal("Response is hashable")
	}
}

func TestModuleMembers(t *testing.T) {
	m := Must(Execute()).(*object.Module)
	want := []string{"Client", "Headers", "Request", "Response", "StatusError", "exchange", "get", "multipart", "request", "send"}
	var got []string
	for name := range m.Members {
		got = append(got, name)
	}
	if strings.Join(sortStrings(got), " ") != strings.Join(want, " ") {
		t.Fatalf("members = %v", sortStrings(got))
	}
	for _, removed := range []string{"post", "put", "patch", "delete"} {
		if _, err := m.GetAttr(removed); err == nil {
			t.Errorf("http.%s exists", removed)
		}
	}
	c := newClientT(t)
	if strings.Join(c.Attributes(), " ") != "exchange send request get cookies" {
		t.Fatalf("Client attributes = %v", c.Attributes())
	}
	if c.(*Client).String() != "<Client>" {
		t.Fatal("Client Show")
	}
	if userAgent != "goblin/"+object.Version {
		t.Fatalf("User-Agent = %q", userAgent)
	}
}
