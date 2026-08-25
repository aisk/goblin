// Package netip adapts Go's net/netip value types to Goblin.
package netip

import (
	"net/netip"

	"github.com/aisk/goblin/object"
)

var (
	addrType   = object.NewNativeConstructor("Addr", addrConstructor)
	prefixType = object.NewNativeConstructor("Prefix", prefixConstructor)
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "netip", Members: map[string]object.Object{
		"Addr":   addrType.Function,
		"Prefix": prefixType.Function,
	}}, nil
}

func addrConstructor(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Addr", args)
	value := p.Any("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if addr, ok := value.(*Addr); ok {
		return addr, nil
	}
	text, ok := value.(object.String)
	if !ok {
		return nil, object.NewTypeError("Addr() argument 'value' must be Addr or str, got %s", value.TypeName())
	}
	addr, err := netip.ParseAddr(string(text))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "Addr() invalid IP address", err)
	}
	return newAddr(addr), nil
}

func prefixConstructor(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Prefix", args)
	value := p.Any("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if prefix, ok := value.(*Prefix); ok {
		return prefix, nil
	}
	text, ok := value.(object.String)
	if !ok {
		return nil, object.NewTypeError("Prefix() argument 'value' must be Prefix or str, got %s", value.TypeName())
	}
	prefix, err := netip.ParsePrefix(string(text))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "Prefix() invalid IP prefix", err)
	}
	return newPrefix(prefix), nil
}
