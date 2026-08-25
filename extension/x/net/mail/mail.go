package mail

import (
	stdmail "net/mail"

	"github.com/aisk/goblin/object"
)

var addressType = object.NewNativeConstructor("Address", addressConstructor)

func Execute() (object.Object, error) {
	return &object.Module{Name: "mail", Members: map[string]object.Object{
		"Address":            addressType.Function,
		"parse_address":      &object.Function{Name: "parse_address", Fn: parseAddress},
		"parse_address_list": &object.Function{Name: "parse_address_list", Fn: parseAddressList},
	}}, nil
}

func addressConstructor(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Address", args)
	name := p.Str("name")
	address := p.Str("address")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return NewAddress(&stdmail.Address{Name: string(name), Address: string(address)}), nil
}

func parseAddress(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("parse_address", args)
	value := p.Str("s")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	address, err := stdmail.ParseAddress(string(value))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "parse_address() invalid mail address", err)
	}
	return NewAddress(address), nil
}

func parseAddressList(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("parse_address_list", args)
	value := p.Str("s")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	addresses, err := stdmail.ParseAddressList(string(value))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "parse_address_list() invalid mail address list", err)
	}
	values := make([]object.Object, len(addresses))
	for i, address := range addresses {
		values[i] = NewAddress(address)
	}
	return &object.List{Elements: values}, nil
}
