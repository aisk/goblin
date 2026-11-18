package http

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func newHeadersT(t *testing.T, fields ...object.Object) *Headers {
	t.Helper()
	return call(t, headersType.Function, fields).(*Headers)
}

func TestHeadersConstruction(t *testing.T) {
	if h := newHeadersT(t); h.String() != "Headers({})" {
		t.Fatalf("Headers() = %v", h)
	}
	if h := newHeadersT(t, object.Nil); h.String() != "Headers({})" {
		t.Fatalf("Headers(nil) = %v", h)
	}
	src := dict(str("Content-Type"), str("text/html"), str("Set-Cookie"), list(str("a=1"), str("b=2")), str("set-cookie"), str("c=3"))
	h := newHeadersT(t, src)
	if got := h.String(); got != `Headers({"content-type": "text/html", "set-cookie": ["a=1", "b=2", "c=3"]})` {
		t.Fatalf("Show = %s", got)
	}
	// The Dict is copied.
	src.Set(str("X-Late"), str("1"))
	if h.has("x-late") {
		t.Fatal("Headers aliases the Dict it was built from")
	}
	// A Headers is copied, and the copy is independent.
	copied := newHeadersT(t, h)
	if eq, _ := copied.Equals(h); !eq || copied == h {
		t.Fatal("Headers(headers) is not an equal copy")
	}
	method(t, copied, "add", args(str("set-cookie"), str("d=4")))
	if values, _ := h.lookup("set-cookie"); len(values) != 3 {
		t.Fatal("changing the copy changed the original")
	}

	cases := []struct {
		fields object.Object
		kind   *object.Error
		msg    string
	}{
		{list(list(str("a"), str("b"))), object.TypeError, "Headers() argument 'fields' must be Dict, Headers or Nil, got List"},
		{dict(object.Integer(1), str("x")), object.TypeError, "Headers() argument 'fields' keys must be String, got Int"},
		{dict(str("x-count"), object.Integer(3)), object.TypeError, "Headers() argument 'fields' values must be String or List of String, got Int"},
		{dict(str("x"), list(str("a"), object.Integer(1))), object.TypeError, "Headers() argument 'fields' values must be String or List of String, got List containing Int"},
		{dict(str("a b"), str("x")), object.ValueError, `Headers() invalid header name "a b"`},
		{dict(str(""), str("x")), object.ValueError, `Headers() invalid header name ""`},
		{dict(str("a:b"), str("x")), object.ValueError, `Headers() invalid header name "a:b"`},
		{dict(str("naïve"), str("x")), object.ValueError, `Headers() invalid header name "naïve"`},
		{dict(str("X-Id"), str("a\x01b")), object.ValueError, `Headers() invalid value for header "x-id"`},
		{dict(str("X-Id"), str("a\x7f")), object.ValueError, `Headers() invalid value for header "x-id"`},
		{dict(str("X-Id"), str("a\r\nInjected: 1")), object.ValueError, `Headers() invalid value for header "x-id"`},
		{dict(str("X-Id"), list(str("ok"), str("a\x00"))), object.ValueError, `Headers() invalid value for header "x-id"`},
	}
	for _, c := range cases {
		_, err := callErr(t, headersType.Function, args(c.fields))
		wantErr(t, err, c.kind, c.msg)
	}
	// A tab and non-ASCII bytes are valid in a value.
	tabbed := newHeadersT(t, dict(str("X-Id"), str("a\tb ünï")))
	if v, _ := tabbed.lookup("x-id"); v[0] != "a\tb ünï" {
		t.Fatal("tab not kept")
	}
	// headers= of a function names that function and its argument.
	_, err := callErr(t, requestType.Function, args(str("GET"), str("http://x.test")), kw("headers", dict(str("X-Count"), object.Integer(3))))
	wantErr(t, err, object.TypeError, "Request() argument 'headers' values must be String or List of String, got Int")
	_, err = callErr(t, member(t, "request"), args(str("GET"), str("http://x.test")), kw("headers", dict(str("a b"), str("1"))))
	wantErr(t, err, object.ValueError, `request() invalid header name "a b"`)
	_, err = callErr(t, member(t, "get"), args(str("http://x.test")), kw("headers", list()))
	wantErr(t, err, object.TypeError, "get() argument 'headers' must be Dict, Headers or Nil, got List")
	_, err = callErr(t, clientType.Function, nil, kw("headers", dict(str("X"), str("\n"))))
	wantErr(t, err, object.ValueError, `Client() invalid value for header "x"`)
}

func TestHeadersMembers(t *testing.T) {
	h := newHeadersT(t, dict(str("Content-Type"), str("text/html"), str("Set-Cookie"), list(str("a=1; Path=/"), str("b=2, x"))))
	idx := func(name string) (object.Object, error) { return h.Index(str(name)) }
	if v, _ := idx("CONTENT-type"); v != str("text/html") {
		t.Fatalf("h[name] in another case = %v", v)
	}
	if v, _ := idx("set-cookie"); v != str("a=1; Path=/, b=2, x") {
		t.Fatalf("joined = %v", v)
	}
	_, err := idx("X-Absent")
	wantErr(t, err, object.KeyError, "key not found: \"X-Absent\"")
	if _, err := h.Index(object.Integer(1)); err == nil {
		t.Fatal("h[1]")
	}
	if v := method(t, h, "get", args(str("SET-COOKIE"))); v != str("a=1; Path=/, b=2, x") {
		t.Fatalf("get = %v", v)
	}
	if v := method(t, h, "get", args(str("x-absent"))); v != object.Nil {
		t.Fatalf("get absent = %v", v)
	}
	if v := method(t, h, "get", args(str("x-absent")), kw("default", str("d"))); v != str("d") {
		t.Fatalf("get default = %v", v)
	}
	if v := method(t, h, "get_all", args(str("Set-Cookie"))); !modtest.Equal(v, list(str("a=1; Path=/"), str("b=2, x"))) {
		t.Fatalf("get_all = %v", v)
	}
	if v := method(t, h, "get_all", args(str("x-absent"))); !modtest.Equal(v, list()) {
		t.Fatalf("get_all absent = %v", v)
	}
	// add keeps the values and returns the receiver.
	if v := method(t, h, "add", args(str("X-Id"), str("1"))); v != h {
		t.Fatal("add() does not return the receiver")
	}
	method(t, h, "add", args(str("x-ID"), str("2")))
	if v := method(t, h, "get_all", args(str("x-id"))); !modtest.Equal(v, list(str("1"), str("2"))) {
		t.Fatalf("add twice = %v", v)
	}
	_, err = methodErr(t, h, "add", args(str("a b"), str("1")))
	wantErr(t, err, object.ValueError, `add() invalid header name "a b"`)
	_, err = methodErr(t, h, "add", args(str("x"), str("\x01")))
	wantErr(t, err, object.ValueError, `add() invalid value for header "x"`)
	_, err = methodErr(t, h, "add", args(str("x"), list(str("1"))))
	wantErr(t, err, object.TypeError, "add() argument 'value' must be String, got List")
	// Index assignment replaces every value; a List gives several.
	if ok, err := h.SetIndex(str("X-ID"), str("only")); !ok || err != nil {
		t.Fatal(err)
	}
	if v := method(t, h, "get_all", args(str("x-id"))); !modtest.Equal(v, list(str("only"))) {
		t.Fatalf("h[x] = s: %v", v)
	}
	h.SetIndex(str("x-id"), list(str("p"), str("q")))
	if v := method(t, h, "get_all", args(str("x-id"))); !modtest.Equal(v, list(str("p"), str("q"))) {
		t.Fatalf("h[x] = list: %v", v)
	}
	if _, err := h.SetIndex(str("a b"), str("x")); err == nil || err.Error() != `invalid header name "a b"` {
		t.Fatalf("h[bad] = %v", err)
	}
	if _, err := h.SetIndex(str("x"), object.Integer(1)); err == nil || err.Error() != "header value must be String or List of String, got Int" {
		t.Fatalf("h[x] = 1: %v", err)
	}
	// remove: returns the receiver; an absent name is accepted.
	if v := method(t, h, "remove", args(str("X-ID"))); v != h || h.has("x-id") {
		t.Fatal("remove")
	}
	if v := method(t, h, "remove", args(str("never"))); v != h {
		t.Fatal("remove absent")
	}
	if method(t, h, "contains", args(str("CONTENT-TYPE"))) != object.True || method(t, h, "contains", args(str("x-id"))) != object.False {
		t.Fatal("contains")
	}
	if v := method(t, h, "keys", nil); !modtest.Equal(v, list(str("content-type"), str("set-cookie"))) {
		t.Fatalf("keys = %v", v)
	}
	if v := method(t, h, "items", nil); !modtest.Equal(v, list(list(str("content-type"), str("text/html")), list(str("set-cookie"), str("a=1; Path=/")), list(str("set-cookie"), str("b=2, x")))) {
		t.Fatalf("items = %v", v)
	}
	if get(t, h, "size") != object.Integer(2) {
		t.Fatal("size")
	}
	items, err := object.Collect(h)
	if err != nil || !modtest.Equal(list(items...), list(str("content-type"), str("set-cookie"))) {
		t.Fatalf("for name in h = %v %v", items, err)
	}
	if b, _ := h.ToBool(); !b {
		t.Fatal("non-empty Headers is false")
	}
	if b, _ := newHeadersT(t).ToBool(); b {
		t.Fatal("empty Headers is true")
	}
	if _, err := object.Hash(h); err == nil {
		t.Fatal("Headers is hashable")
	}
	if _, err := h.Compare(newHeadersT(t)); err == nil {
		t.Fatal("Headers has an order")
	}
	for _, name := range []string{"values", "copy", "pop", "update", "clear"} {
		if _, err := h.GetAttr(name); err == nil {
			t.Errorf("Headers has %s", name)
		}
	}
}

func TestHeadersEquality(t *testing.T) {
	a := newHeadersT(t, dict(str("A"), str("1"), str("B"), list(str("x"), str("y"))))
	b := newHeadersT(t, dict(str("b"), list(str("x"), str("y")), str("a"), str("1")))
	c := newHeadersT(t, dict(str("a"), str("1"), str("b"), list(str("y"), str("x"))))
	d := newHeadersT(t, dict(str("a"), str("1")))
	if !modtest.Equal(a, b) {
		t.Fatal("the same fields in another name order are unequal")
	}
	if modtest.Equal(a, c) {
		t.Fatal("a different value order is equal")
	}
	if modtest.Equal(a, d) || modtest.Equal(d, a) {
		t.Fatal("a subset is equal")
	}
	if modtest.Equal(d, dict(str("a"), str("1"))) {
		t.Fatal("a Headers equals a Dict")
	}
}

// TestHeadersNeverHoldANameWithoutValues: an empty List of values removes
// the name, by assignment and when building from a Dict.
func TestHeadersNeverHoldANameWithoutValues(t *testing.T) {
	h := newHeadersT(t, dict(str("a"), list(), str("b"), str("1")))
	if h.has("a") || h.String() != `Headers({"b": "1"})` {
		t.Fatalf("Headers({a: [], b: 1}) = %v", h)
	}
	if _, err := h.SetIndex(str("B"), list()); err != nil {
		t.Fatal(err)
	}
	if truth, _ := h.ToBool(); h.has("b") || h.String() != "Headers({})" || truth {
		t.Fatalf("after h[\"B\"] = [] = %v", h)
	}
	if eq, _ := h.Equals(newHeadersT(t)); !eq {
		t.Fatal("an emptied Headers differs from Headers()")
	}
	// Assigning to a name that is not there adds nothing.
	if _, err := h.SetIndex(str("c"), list()); err != nil || h.has("c") {
		t.Fatalf("h[\"c\"] = [] = %v, %v", h, err)
	}
}
