package netip

import (
	"net/netip"

	"github.com/aisk/goblin/object"
)

type Prefix struct {
	object.OpaqueBase
	value netip.Prefix
}

func newPrefix(value netip.Prefix) *Prefix {
	return &Prefix{OpaqueBase: object.MakeOpaqueBase("Prefix"), value: value}
}

func (p *Prefix) String() string              { return p.value.String() }
func (p *Prefix) ToString() (string, error)   { return p.String(), nil }
func (p *Prefix) ToBool() (bool, error)       { return p.value.IsValid(), nil }
func (p *Prefix) Not() (object.Object, error) { return object.Bool(!p.value.IsValid()), nil }
func (p *Prefix) Equals(other object.Object) (bool, error) {
	value, ok := other.(*Prefix)
	return ok && p.value == value.value, nil
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
	if err := object.RequireNoArgs("masked", args); err != nil {
		return nil, err
	}
	return newPrefix(p.value.Masked()), nil
}
func (p *Prefix) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(p), nil
	case "addr":
		return newAddr(p.value.Addr()), nil
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
