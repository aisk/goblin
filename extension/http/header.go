package http

import (
	"fmt"
	stdhttp "net/http"
	"sort"

	"github.com/aisk/goblin/object"
)

// Header wraps net/http.Header, mirroring its Get/Values/Set/Add/Del API.
// Because http.Header is a map (a reference type), a Header shares storage with
// the request or response it was obtained from, so mutations made via
// set/add/del are reflected in the underlying request before it is sent.
type Header struct {
	object.OpaqueBase
	Header stdhttp.Header
}

func NewHeader(h stdhttp.Header) *Header {
	if h == nil {
		h = stdhttp.Header{}
	}
	return &Header{OpaqueBase: object.MakeOpaqueBase("Header"), Header: h}
}

func (h *Header) String() string {
	return fmt.Sprintf("<http_header %d>", len(h.Header))
}

func (h *Header) ToString() (string, error) { return h.String(), nil }

func (h *Header) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(h), nil
	case "get":
		return &object.Function{Name: "get", Fn: h.get}, nil
	case "values":
		return &object.Function{Name: "values", Fn: h.values}, nil
	case "set":
		return &object.Function{Name: "set", Fn: h.set}, nil
	case "add":
		return &object.Function{Name: "add", Fn: h.add}, nil
	case "del":
		return &object.Function{Name: "del", Fn: h.del}, nil
	default:
		return nil, object.NewAttributeError("Header has no attribute '%s'", name)
	}
}

func (h *Header) Attributes() []string {
	return []string{"attributes", "get", "values", "set", "add", "del"}
}

// Index reads h[key] like get(), except that a missing header raises
// KeyError, as a Dict lookup does.
func (h *Header) Index(key object.Object) (object.Object, error) {
	name, ok := key.(object.String)
	if !ok {
		return nil, object.NewTypeError("Header index must be str, got %s", key.TypeName())
	}
	values := h.Header.Values(string(name))
	if len(values) == 0 {
		return nil, object.NewKeyError("key not found: %s", string(name))
	}
	return object.String(values[0]), nil
}

// SetIndex makes h[key] = value the same as set(key, value).
func (h *Header) SetIndex(key, value object.Object) (bool, error) {
	name, ok := key.(object.String)
	if !ok {
		return true, object.NewTypeError("Header index must be str, got %s", key.TypeName())
	}
	text, ok := value.(object.String)
	if !ok {
		return true, object.NewTypeError("Header value must be str, got %s", value.TypeName())
	}
	h.Header.Set(string(name), string(text))
	return true, nil
}

// Iter yields the header names in canonical form, sorted so iteration is
// deterministic.
func (h *Header) Iter() ([]object.Object, error) {
	names := make([]string, 0, len(h.Header))
	for name := range h.Header {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]object.Object, len(names))
	for i, name := range names {
		items[i] = object.String(name)
	}
	return items, nil
}

// get returns the first value associated with the given key, or "" if none.
func (h *Header) get(args object.CallArgs) (object.Object, error) {
	key, err := headerKeyArg("get", args)
	if err != nil {
		return nil, err
	}
	return object.String(h.Header.Get(key)), nil
}

// values returns all values associated with the given key as a list.
func (h *Header) values(args object.CallArgs) (object.Object, error) {
	key, err := headerKeyArg("values", args)
	if err != nil {
		return nil, err
	}
	vals := h.Header.Values(key)
	elements := make([]object.Object, 0, len(vals))
	for _, v := range vals {
		elements = append(elements, object.String(v))
	}
	return &object.List{Elements: elements}, nil
}

// set replaces the values associated with key with the single value.
func (h *Header) set(args object.CallArgs) (object.Object, error) {
	key, value, err := headerKeyValueArgs("set", args)
	if err != nil {
		return nil, err
	}
	h.Header.Set(key, value)
	return object.Nil, nil
}

// add appends a value to the values associated with key.
func (h *Header) add(args object.CallArgs) (object.Object, error) {
	key, value, err := headerKeyValueArgs("add", args)
	if err != nil {
		return nil, err
	}
	h.Header.Add(key, value)
	return object.Nil, nil
}

// del deletes all values associated with key.
func (h *Header) del(args object.CallArgs) (object.Object, error) {
	key, err := headerKeyArg("del", args)
	if err != nil {
		return nil, err
	}
	h.Header.Del(key)
	return object.Nil, nil
}

func headerKeyArg(fn string, args object.CallArgs) (string, error) {
	p := object.NewArgParser(fn, args)
	key := p.Str("key")
	if err := p.Finish(); err != nil {
		return "", err
	}
	return string(key), nil
}

func headerKeyValueArgs(fn string, args object.CallArgs) (string, string, error) {
	p := object.NewArgParser(fn, args)
	key, value := p.Str("key"), p.Str("value")
	if err := p.Finish(); err != nil {
		return "", "", err
	}
	return string(key), string(value), nil
}

var _ object.Object = (*Header)(nil)
