package netip

import (
	"net/netip"

	"github.com/aisk/goblin/object"
)

type Prefix struct {
	object.NoReflectedOps
	object.NoAssignment
	value netip.Prefix
}

func (p *Prefix) TypeName() string            { return "Prefix" }
func (p *Prefix) String() string              { return p.value.String() }
func (p *Prefix) ToString() (string, error)   { return p.String(), nil }
func (p *Prefix) ToBool() (bool, error)       { return p.value.IsValid(), nil }
func (p *Prefix) Not() (object.Object, error) { return object.Bool(!p.value.IsValid()), nil }
func (p *Prefix) Equals(other object.Object) (bool, error) {
	value, ok := other.(*Prefix)
	return ok && p.value == value.value, nil
}
func (p *Prefix) Compare(object.Object) (int, error) {
	return 0, object.NewTypeError("Prefix values are not ordered")
}
func (p *Prefix) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Prefix does not support addition")
}
func (p *Prefix) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Prefix does not support subtraction")
}
func (p *Prefix) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Prefix does not support multiplication")
}
func (p *Prefix) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Prefix does not support division")
}
func (p *Prefix) Modulo(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Prefix does not support modulo")
}
func (p *Prefix) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("Prefix does not support iteration")
}
func (p *Prefix) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Prefix is not indexable")
}

func (p *Prefix) contains(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("contains", args)
	value := ap.Any("addr")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	addr, ok := value.(*Addr)
	if !ok {
		return nil, object.NewTypeError("contains() argument 'addr' must be Addr, got %s", value.TypeName())
	}
	return object.Bool(p.value.Contains(addr.value)), nil
}
func (p *Prefix) overlaps(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("overlaps", args)
	value := ap.Any("prefix")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	other, ok := value.(*Prefix)
	if !ok {
		return nil, object.NewTypeError("overlaps() argument 'prefix' must be Prefix, got %s", value.TypeName())
	}
	return object.Bool(p.value.Overlaps(other.value)), nil
}
func (p *Prefix) masked(args object.CallArgs) (object.Object, error) {
	if err := noArgs("masked", args); err != nil {
		return nil, err
	}
	return &Prefix{value: p.value.Masked()}, nil
}
func (p *Prefix) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(p), nil
	case "addr":
		return &Addr{value: p.value.Addr()}, nil
	case "bits":
		return object.Integer(p.value.Bits()), nil
	case "is_single_ip":
		return object.Bool(p.value.IsSingleIP()), nil
	case "contains":
		return &object.Function{Name: "contains", Fn: p.contains}, nil
	case "overlaps":
		return &object.Function{Name: "overlaps", Fn: p.overlaps}, nil
	case "masked":
		return &object.Function{Name: "masked", Fn: p.masked}, nil
	default:
		return nil, object.NewAttributeError("Prefix has no attribute '%s'", name)
	}
}
func (p *Prefix) Attributes() []string {
	return []string{"attributes", "addr", "bits", "is_single_ip", "contains", "overlaps", "masked"}
}

var _ object.Object = (*Prefix)(nil)
