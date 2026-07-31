package netip

import (
	"net/netip"

	"github.com/aisk/goblin/object"
)

type Addr struct {
	object.NoReflectedOps
	object.NoAssignment
	value netip.Addr
}

func (a *Addr) TypeName() string            { return "Addr" }
func (a *Addr) String() string              { return a.value.String() }
func (a *Addr) ToString() (string, error)   { return a.String(), nil }
func (a *Addr) ToBool() (bool, error)       { return a.value.IsValid(), nil }
func (a *Addr) Not() (object.Object, error) { return object.Bool(!a.value.IsValid()), nil }
func (a *Addr) Equals(other object.Object) (bool, error) {
	value, ok := other.(*Addr)
	return ok && a.value == value.value, nil
}
func (a *Addr) Compare(other object.Object) (int, error) {
	value, ok := other.(*Addr)
	if !ok {
		return 0, object.NewTypeError("cannot compare Addr with %s", other.TypeName())
	}
	return a.value.Compare(value.value), nil
}
func (a *Addr) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Addr does not support addition")
}
func (a *Addr) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Addr does not support subtraction")
}
func (a *Addr) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Addr does not support multiplication")
}
func (a *Addr) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Addr does not support division")
}
func (a *Addr) Modulo(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Addr does not support modulo")
}
func (a *Addr) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("Addr does not support iteration")
}
func (a *Addr) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Addr is not indexable")
}

func noArgs(name string, args object.CallArgs) error { return object.NewArgParser(name, args).Finish() }
func (a *Addr) next(args object.CallArgs) (object.Object, error) {
	if err := noArgs("next", args); err != nil {
		return nil, err
	}
	value := a.value.Next()
	if !value.IsValid() {
		return nil, object.NewValueError("next() address has no successor")
	}
	return &Addr{value: value}, nil
}
func (a *Addr) prev(args object.CallArgs) (object.Object, error) {
	if err := noArgs("prev", args); err != nil {
		return nil, err
	}
	value := a.value.Prev()
	if !value.IsValid() {
		return nil, object.NewValueError("prev() address has no predecessor")
	}
	return &Addr{value: value}, nil
}
func (a *Addr) unmap(args object.CallArgs) (object.Object, error) {
	if err := noArgs("unmap", args); err != nil {
		return nil, err
	}
	return &Addr{value: a.value.Unmap()}, nil
}

func (a *Addr) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(a), nil
	case "bits":
		return object.Integer(a.value.BitLen()), nil
	case "bytes":
		value := a.value.AsSlice()
		return object.NewBytes(value), nil
	case "zone":
		return object.String(a.value.Zone()), nil
	case "is4":
		return object.Bool(a.value.Is4()), nil
	case "is6":
		return object.Bool(a.value.Is6()), nil
	case "is_loopback":
		return object.Bool(a.value.IsLoopback()), nil
	case "is_multicast":
		return object.Bool(a.value.IsMulticast()), nil
	case "is_private":
		return object.Bool(a.value.IsPrivate()), nil
	case "is_unspecified":
		return object.Bool(a.value.IsUnspecified()), nil
	case "is_link_local_unicast":
		return object.Bool(a.value.IsLinkLocalUnicast()), nil
	case "is_link_local_multicast":
		return object.Bool(a.value.IsLinkLocalMulticast()), nil
	case "next":
		return &object.Function{Name: "next", Fn: a.next}, nil
	case "prev":
		return &object.Function{Name: "prev", Fn: a.prev}, nil
	case "unmap":
		return &object.Function{Name: "unmap", Fn: a.unmap}, nil
	default:
		return nil, object.NewAttributeError("Addr has no attribute '%s'", name)
	}
}
func (a *Addr) Attributes() []string {
	return []string{"attributes", "bits", "bytes", "zone", "is4", "is6", "is_loopback", "is_multicast", "is_private", "is_unspecified", "is_link_local_unicast", "is_link_local_multicast", "next", "prev", "unmap"}
}

var _ object.Object = (*Addr)(nil)
