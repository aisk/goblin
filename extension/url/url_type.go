package url

import (
	"net/url"

	"github.com/aisk/goblin/object"
)

type URL struct {
	value *url.URL
}

func (u *URL) String() string            { return u.value.String() }
func (u *URL) ToString() (string, error) { return u.value.String(), nil }
func (u *URL) Bool() bool                { return true }
func (u *URL) ToBool() (bool, error)     { return true, nil }
func (u *URL) Equals(other object.Object) bool {
	value, ok := other.(*URL)
	return ok && u.value.String() == value.value.String()
}
func (u *URL) Compare(object.Object) (int, error) {
	return 0, object.NewTypeError("URL values are not ordered")
}
func (u *URL) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("URL does not support addition")
}
func (u *URL) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("URL does not support subtraction")
}
func (u *URL) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("URL does not support multiplication")
}
func (u *URL) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("URL does not support division")
}
func (u *URL) Not() (object.Object, error) { return object.False, nil }
func (u *URL) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("URL does not support iteration")
}
func (u *URL) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("URL is not indexable")
}

func (u *URL) resolveReference(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("resolve_reference", args)
	reference := p.Any("reference")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	other, ok := reference.(*URL)
	if !ok {
		return nil, object.NewTypeError("resolve_reference() argument 'reference' must be a URL, got %T", reference)
	}
	return &URL{value: u.value.ResolveReference(other.value)}, nil
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
