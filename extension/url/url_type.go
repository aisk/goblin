package url

import (
	"net/url"

	"github.com/aisk/goblin/object"
)

type URL struct {
	object.OpaqueBase
	value *url.URL
}

func newURL(value *url.URL) *URL {
	return &URL{OpaqueBase: object.MakeOpaqueBase("URL"), value: value}
}

func (u *URL) String() string            { return u.value.String() }
func (u *URL) ToString() (string, error) { return u.value.String(), nil }
func (u *URL) Equals(other object.Object) (bool, error) {
	value, ok := other.(*URL)
	return ok && u.value.String() == value.value.String(), nil
}

func (u *URL) resolveReference(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("resolve_reference", args)
	reference := p.Any("reference")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	other, ok := reference.(*URL)
	if !ok {
		return nil, object.NewTypeError("resolve_reference() argument 'reference' must be a URL, got %s", reference.TypeName())
	}
	return newURL(u.value.ResolveReference(other.value)), nil
}

func (u *URL) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(u), nil
	case "scheme":
		return object.String(u.value.Scheme), nil
	case "host":
		return object.String(u.value.Host), nil
	case "path":
		return object.String(u.value.Path), nil
	case "raw_query":
		return object.String(u.value.RawQuery), nil
	case "fragment":
		return object.String(u.value.Fragment), nil
	case "hostname":
		return object.String(u.value.Hostname()), nil
	case "port":
		return object.String(u.value.Port()), nil
	case "escaped_path":
		return object.String(u.value.EscapedPath()), nil
	case "resolve_reference":
		return &object.Function{Name: "resolve_reference", Fn: u.resolveReference}, nil
	}
	return nil, object.NewAttributeError("URL has no attribute '%s'", name)
}

func (u *URL) Attributes() []string {
	return []string{"attributes", "scheme", "host", "path", "raw_query", "fragment", "hostname", "port", "escaped_path", "resolve_reference"}
}

var _ object.Object = (*URL)(nil)
