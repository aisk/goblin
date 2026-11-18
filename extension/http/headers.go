package http

import (
	"net/textproto"
	"sort"
	"strings"

	stdhttp "net/http"

	"github.com/aisk/goblin/object"
)

// headersType is http.Headers, the constructor and what type_of answers.
var headersType = object.NewNativeConstructor("Headers", newHeaders)

// Headers is the header fields of one message: names case-insensitive and
// stored in lower case, several values per name, names in the order they
// were first added. It is mutable, like a Dict, and like a Dict has no lock.
type Headers struct {
	object.OpaqueBase
	fields []headerField
	index  map[string]int
}

type headerField struct {
	name   string
	values []string
}

var _ object.Object = (*Headers)(nil)

func emptyHeaders() *Headers {
	return &Headers{OpaqueBase: object.MakeOpaqueBase("Headers"), index: map[string]int{}}
}

// newHeaders is Headers(fields=nil).
func newHeaders(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Headers", args)
	fields := p.AnyOr("fields", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return headersFrom("Headers", "fields", fields)
}

// headersFrom is what every headers= parameter (and Headers()) accepts:
// nil, a Dict of String to String or List of String, or a Headers. It
// validates the names and values and always returns a new Headers, so the
// caller's value is never aliased.
func headersFrom(fn, param string, v object.Object) (*Headers, error) {
	h := emptyHeaders()
	switch value := v.(type) {
	case object.Unit:
		return h, nil
	case *Headers:
		for _, f := range value.fields {
			h.append(f.name, f.values...)
		}
		return h, nil
	case *object.Dict:
		for _, entry := range value.Entries() {
			name, ok := entry.Key.(object.String)
			if !ok {
				return nil, object.NewTypeError("%s() argument '%s' keys must be String, got %s", fn, param, entry.Key.TypeName())
			}
			values, err := fieldValues(entry.Value)
			if err != nil {
				return nil, object.NewTypeError("%s() argument '%s' values must be String or List of String, got %s", fn, param, err.Error())
			}
			if err := checkField(fn+"() ", string(name), values); err != nil {
				return nil, err
			}
			h.append(string(name), values...)
		}
		return h, nil
	}
	return nil, object.NewTypeError("%s() argument '%s' must be Dict, Headers or Nil, got %s", fn, param, v.TypeName())
}

// typeError is the description of a wrong value type in a TypeError.
type typeError string

func (e typeError) Error() string { return string(e) }

// fieldValues reads a header value: a String, or a List of String.
func fieldValues(v object.Object) ([]string, error) {
	switch value := v.(type) {
	case object.String:
		return []string{string(value)}, nil
	case *object.List:
		out := make([]string, len(value.Elements))
		for i, item := range value.Elements {
			s, ok := item.(object.String)
			if !ok {
				return nil, typeError("List containing " + item.TypeName())
			}
			out[i] = string(s)
		}
		return out, nil
	}
	return nil, typeError(v.TypeName())
}

// checkField validates a name and its values. prefix starts the message:
// "fn() " for a call, "" for index assignment, which is syntax.
func checkField(prefix, name string, values []string) error {
	if !validName(name) {
		return object.NewValueError("%sinvalid header name %q", prefix, name)
	}
	for _, v := range values {
		if !validValue(v) {
			return object.NewValueError("%sinvalid value for header %q", prefix, strings.ToLower(name))
		}
	}
	return nil
}

// validName reports whether name is an RFC 9110 token: one or more of
// !#$%&'*+-.^_`|~, digits and letters.
func validName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isTchar(name[i]) {
			return false
		}
	}
	return true
}

func isTchar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0
}

// validValue is httpguts.ValidHeaderFieldValue, which Go's transport
// applies: no control character other than tab, and no DEL.
func validValue(v string) bool {
	for i := 0; i < len(v); i++ {
		c := v[i]
		if (c < 0x20 && c != '\t') || c == 0x7f {
			return false
		}
	}
	return true
}

// append adds values to name, creating it at the end when it is new. No
// values add nothing: a name is never held without a value.
func (h *Headers) append(name string, values ...string) {
	if len(values) == 0 {
		return
	}
	key := strings.ToLower(name)
	if i, ok := h.index[key]; ok {
		h.fields[i].values = append(h.fields[i].values, values...)
		return
	}
	h.index[key] = len(h.fields)
	h.fields = append(h.fields, headerField{name: key, values: append([]string(nil), values...)})
}

// replace sets every value of name, keeping its position when it exists.
// No values remove the name, as h[name] = [] says.
func (h *Headers) replace(name string, values []string) {
	if len(values) == 0 {
		h.remove(name)
		return
	}
	key := strings.ToLower(name)
	if i, ok := h.index[key]; ok {
		h.fields[i].values = append([]string(nil), values...)
		return
	}
	h.append(key, values...)
}

func (h *Headers) remove(name string) {
	key := strings.ToLower(name)
	i, ok := h.index[key]
	if !ok {
		return
	}
	h.fields = append(h.fields[:i], h.fields[i+1:]...)
	delete(h.index, key)
	for j := i; j < len(h.fields); j++ {
		h.index[h.fields[j].name] = j
	}
}

// lookup returns the values of name in any case.
func (h *Headers) lookup(name string) ([]string, bool) {
	i, ok := h.index[strings.ToLower(name)]
	if !ok {
		return nil, false
	}
	return h.fields[i].values, true
}

func (h *Headers) has(name string) bool {
	_, ok := h.lookup(name)
	return ok
}

// toGo converts to a Go header with canonical names.
func (h *Headers) toGo() stdhttp.Header {
	out := make(stdhttp.Header, len(h.fields))
	for _, f := range h.fields {
		key := textproto.CanonicalMIMEHeaderKey(f.name)
		out[key] = append(out[key], f.values...)
	}
	return out
}

// fromGo converts a received Go header: names in alphabetical order, since
// Go's parser keeps them in a map, and each name's values in the order they
// were sent.
func fromGo(header stdhttp.Header) *Headers {
	names := make([]string, 0, len(header))
	for name := range header {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	h := emptyHeaders()
	for _, name := range names {
		h.append(name, header[name]...)
	}
	return h
}

func (h *Headers) Constructor() *object.Function { return headersType.Function }

func (h *Headers) String() string {
	d := object.NewDict()
	for _, f := range h.fields {
		if len(f.values) == 1 {
			d.Set(object.String(f.name), object.String(f.values[0]))
			continue
		}
		d.Set(object.String(f.name), stringList(f.values))
	}
	return "Headers(" + d.String() + ")"
}

func (h *Headers) ToString() (string, error) { return h.String(), nil }

// ToBool is false when there are no fields, as for a Dict.
func (h *Headers) ToBool() (bool, error) { return len(h.fields) > 0, nil }

// Equals: the same names, each with the same values in the same order; the
// order of different names does not matter. A Headers never equals a Dict.
func (h *Headers) Equals(other object.Object) (bool, error) {
	o, ok := other.(*Headers)
	if !ok || len(o.fields) != len(h.fields) {
		return false, nil
	}
	for _, f := range h.fields {
		values, ok := o.lookup(f.name)
		if !ok || len(values) != len(f.values) {
			return false, nil
		}
		for i := range values {
			if values[i] != f.values[i] {
				return false, nil
			}
		}
	}
	return true, nil
}

// Index is h[name]: the value, or the values joined with ", ".
func (h *Headers) Index(index object.Object) (object.Object, error) {
	name, ok := index.(object.String)
	if !ok {
		return nil, object.NewTypeError("header name must be String, got %s", index.TypeName())
	}
	values, ok := h.lookup(string(name))
	if !ok {
		return nil, object.NewKeyError("key not found: %q", string(name))
	}
	return object.String(strings.Join(values, ", ")), nil
}

// SetIndex is h[name] = value: it replaces every value of name.
func (h *Headers) SetIndex(index object.Object, value object.Object) (bool, error) {
	name, ok := index.(object.String)
	if !ok {
		return true, object.NewTypeError("header name must be String, got %s", index.TypeName())
	}
	values, err := fieldValues(value)
	if err != nil {
		return true, object.NewTypeError("header value must be String or List of String, got %s", err.Error())
	}
	if err := checkField("", string(name), values); err != nil {
		return true, err
	}
	h.replace(string(name), values)
	return true, nil
}

// Iter gives the names, as keys() does.
func (h *Headers) Iter() ([]object.Object, error) { return h.names().Elements, nil }

func (h *Headers) names() *object.List {
	out := make([]object.Object, len(h.fields))
	for i, f := range h.fields {
		out[i] = object.String(f.name)
	}
	return &object.List{Elements: out}
}

func stringList(values []string) *object.List {
	out := make([]object.Object, len(values))
	for i, v := range values {
		out[i] = object.String(v)
	}
	return &object.List{Elements: out}
}

func (h *Headers) get(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("get", args)
	name := p.Str("name")
	def := p.AnyOr("default", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	values, ok := h.lookup(string(name))
	if !ok {
		return def, nil
	}
	return object.String(strings.Join(values, ", ")), nil
}

func (h *Headers) getAll(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("get_all", args)
	name := p.Str("name")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	values, _ := h.lookup(string(name))
	return stringList(values), nil
}

func (h *Headers) add(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("add", args)
	name := p.Str("name")
	value := p.Str("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if err := checkField("add() ", string(name), []string{string(value)}); err != nil {
		return nil, err
	}
	h.append(string(name), string(value))
	return h, nil
}

func (h *Headers) removeMethod(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("remove", args)
	name := p.Str("name")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	h.remove(string(name))
	return h, nil
}

func (h *Headers) contains(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("contains", args)
	name := p.Str("name")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Bool(h.has(string(name))), nil
}

func (h *Headers) keys(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("keys", args); err != nil {
		return nil, err
	}
	return h.names(), nil
}

func (h *Headers) items(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("items", args); err != nil {
		return nil, err
	}
	var out []object.Object
	for _, f := range h.fields {
		for _, v := range f.values {
			out = append(out, &object.List{Elements: []object.Object{object.String(f.name), object.String(v)}})
		}
	}
	if out == nil {
		out = []object.Object{}
	}
	return &object.List{Elements: out}, nil
}

func (h *Headers) GetAttr(name string) (object.Object, error) {
	switch name {
	case "size":
		return object.Integer(len(h.fields)), nil
	case "get":
		return &object.Function{Name: "get", Fn: h.get}, nil
	case "get_all":
		return &object.Function{Name: "get_all", Fn: h.getAll}, nil
	case "add":
		return &object.Function{Name: "add", Fn: h.add}, nil
	case "remove":
		return &object.Function{Name: "remove", Fn: h.removeMethod}, nil
	case "contains":
		return &object.Function{Name: "contains", Fn: h.contains}, nil
	case "keys":
		return &object.Function{Name: "keys", Fn: h.keys}, nil
	case "items":
		return &object.Function{Name: "items", Fn: h.items}, nil
	}
	return nil, object.NewAttributeError("Headers has no attribute '%s'", name)
}

func (h *Headers) Attributes() []string {
	return []string{"size", "get", "get_all", "add", "remove", "contains", "keys", "items"}
}
