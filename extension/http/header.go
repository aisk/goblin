package http

import (
	"fmt"
	stdhttp "net/http"

	"github.com/aisk/goblin/object"
)

// Header wraps net/http.Header, mirroring its Get/Values/Set/Add/Del API.
// Because http.Header is a map (a reference type), a Header shares storage with
// the request or response it was obtained from, so mutations made via
// set/add/del are reflected in the underlying request before it is sent.
type Header struct {
	objectBase
	Header stdhttp.Header
}

func NewHeader(h stdhttp.Header) *Header {
	if h == nil {
		h = stdhttp.Header{}
	}
	return &Header{objectBase: objectBase{typeName: "Header"}, Header: h}
}

func (h *Header) String() string {
	return fmt.Sprintf("<http_header %d>", len(h.Header))
}

func (h *Header) GetAttr(name string) (object.Object, error) {
	switch name {
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
	if err := object.RequireNoKeyword(fn, args); err != nil {
		return "", err
	}
	if len(args.Positional) != 1 {
		return "", object.NewTypeError("%s() takes exactly 1 argument, got %d", fn, len(args.Positional))
	}
	return stringArg(fn, "key", args.Positional[0])
}

func headerKeyValueArgs(fn string, args object.CallArgs) (string, string, error) {
	if err := object.RequireNoKeyword(fn, args); err != nil {
		return "", "", err
	}
	if len(args.Positional) != 2 {
		return "", "", object.NewTypeError("%s() takes exactly 2 arguments, got %d", fn, len(args.Positional))
	}
	key, err := stringArg(fn, "key", args.Positional[0])
	if err != nil {
		return "", "", err
	}
	value, err := stringArg(fn, "value", args.Positional[1])
	if err != nil {
		return "", "", err
	}
	return key, value, nil
}

var _ object.Object = (*Header)(nil)
