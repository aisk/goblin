// Package urlvalue holds the URL value type and the query encoding shared by
// the modules that name things on the network (url, http).
package urlvalue

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/aisk/goblin/object"
)

// Type is the type name url exports for URL. A URL comes from url.parse.
var Type = object.NewOpaqueConstructor("URL", "url.parse(text)")

// URL is a parsed URL: an immutable value that never touches the network.
type URL struct {
	object.OpaqueBase
	u *url.URL
}

var _ object.Object = (*URL)(nil)

// New wraps a Go URL; the URL keeps its own copy.
func New(u *url.URL) *URL {
	c := *u
	if u.User != nil {
		user := *u.User
		c.User = &user
	}
	return &URL{OpaqueBase: object.MakeOpaqueBase("URL"), u: &c}
}

// Go returns a copy of the Go URL, for a caller that changes it.
func (u *URL) Go() *url.URL {
	c := *u.u
	return &c
}

// Parse parses text as fn() does: an invalid URL is a ParseError
// `fn() invalid URL "text": reason` carrying text.
func Parse(fn, text string) (*URL, error) {
	u, err := url.Parse(text)
	if err != nil {
		return nil, invalid(fn, text, err)
	}
	return New(u), nil
}

func invalid(fn, text string, err error) *object.Error {
	reason := err
	var ue *url.Error
	if errors.As(err, &ue) {
		reason = ue.Err
	}
	return object.NewParseError("%s() invalid URL %s: %s", fn, strconv.Quote(text), reason).
		With("text", object.String(text))
}

// FromObject returns the Go URL a `url` argument names: a String parsed
// as fn() parses it, or a URL. Anything else is the standard TypeError.
func FromObject(fn, param string, v object.Object) (*url.URL, error) {
	switch value := v.(type) {
	case object.String:
		u, err := Parse(fn, string(value))
		if err != nil {
			return nil, err
		}
		return u.u, nil
	case *URL:
		return value.Go(), nil
	}
	return nil, object.NewTypeError("%s() argument '%s' must be String or URL, got %s", fn, param, v.TypeName())
}

func (u *URL) Constructor() *object.Function { return Type.Function }
func (u *URL) String() string                { return u.u.String() }
func (u *URL) ToString() (string, error)     { return u.u.String(), nil }

// Equals compares the text; a URL never equals a String.
func (u *URL) Equals(other object.Object) (bool, error) {
	v, ok := other.(*URL)
	return ok && u.u.String() == v.u.String(), nil
}

// Hash agrees with Equals.
func (u *URL) Hash() (uint64, error) { return object.String(u.u.String()).Hash() }

func (u *URL) port() object.Object {
	port := u.u.Port()
	if port == "" {
		return object.Nil
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return object.Nil
	}
	return object.Integer(n)
}

func (u *URL) join(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("join", args)
	reference := p.Any("reference")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	ref, err := FromObject("join", "reference", reference)
	if err != nil {
		return nil, err
	}
	return New(u.u.ResolveReference(ref)), nil
}

func (u *URL) withQuery(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("with_query", args)
	params := p.Dict("params")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	query, err := EncodeQuery("with_query", "params", params)
	if err != nil {
		return nil, err
	}
	c := u.Go()
	c.RawQuery = query
	c.ForceQuery = false
	return New(c), nil
}

func (u *URL) GetAttr(name string) (object.Object, error) {
	switch name {
	case "scheme":
		return object.String(u.u.Scheme), nil
	case "host":
		return object.String(u.u.Hostname()), nil
	case "port":
		return u.port(), nil
	case "path":
		return object.String(u.u.Path), nil
	case "query":
		return object.String(u.u.RawQuery), nil
	case "fragment":
		return object.String(u.u.Fragment), nil
	case "join":
		return &object.Function{Name: "join", Fn: u.join}, nil
	case "with_query":
		return &object.Function{Name: "with_query", Fn: u.withQuery}, nil
	}
	return nil, object.NewAttributeError("URL has no attribute '%s'", name)
}

func (u *URL) Attributes() []string {
	return []string{"scheme", "host", "port", "path", "query", "fragment", "join", "with_query"}
}

// Fields walks a Dict of named fields the way every Dict-to-fields writer
// does (a query, a form, headers): String keys in the Dict's order, a List
// value repeating the name once per element, a nil value leaving the name
// out. Each scalar value is passed to emit as its text; scalars are String,
// Int, Float and Bool. fn and param name the argument in a TypeError.
func Fields(fn, param string, d *object.Dict, emit func(name, value string)) error {
	for _, entry := range d.Entries() {
		key, ok := entry.Key.(object.String)
		if !ok {
			return object.NewTypeError("%s() argument '%s' keys must be String, got %s", fn, param, entry.Key.TypeName())
		}
		values := []object.Object{entry.Value}
		if list, ok := entry.Value.(*object.List); ok {
			values = list.Elements
		}
		for _, v := range values {
			if v == object.Nil {
				continue
			}
			text, ok := scalarText(v)
			if !ok {
				return object.NewTypeError("%s() argument '%s' values must be String, Int, Float, Bool, List or Nil, got %s", fn, param, v.TypeName())
			}
			emit(string(key), text)
		}
	}
	return nil
}

func scalarText(v object.Object) (string, bool) {
	switch v.(type) {
	case object.String, object.Integer, object.Float, object.Bool:
		text, err := v.ToString()
		return text, err == nil
	}
	return "", false
}

// EncodeQuery is url.encode_query: the fields of d as a query string
// without the leading "?", spaces as "+".
func EncodeQuery(fn, param string, d *object.Dict) (string, error) {
	var b strings.Builder
	err := Fields(fn, param, d, func(name, value string) {
		if b.Len() > 0 {
			b.WriteByte('&')
		}
		b.WriteString(url.QueryEscape(name))
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(value))
	})
	return b.String(), err
}

// DecodeQuery is url.decode_query: the names and values of a query string
// in the order of first appearance, a repeated name keeping its last value.
func DecodeQuery(fn, text string) (*object.Dict, error) {
	d := object.NewDict()
	for _, part := range strings.Split(strings.TrimPrefix(text, "?"), "&") {
		if part == "" {
			continue
		}
		rawName, rawValue, _ := strings.Cut(part, "=")
		name, err := url.QueryUnescape(rawName)
		if err != nil {
			return nil, badEscape(fn, text, err)
		}
		value, err := url.QueryUnescape(rawValue)
		if err != nil {
			return nil, badEscape(fn, text, err)
		}
		if err := d.Set(object.String(name), object.String(value)); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func badEscape(fn, text string, err error) error {
	var escape url.EscapeError
	if errors.As(err, &escape) {
		return object.NewParseError("%s() invalid escape %s", fn, strconv.Quote(string(escape))).
			With("text", object.String(text))
	}
	return object.NewParseError("%s() invalid query: %s", fn, err).With("text", object.String(text))
}
