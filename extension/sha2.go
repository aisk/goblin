package extension

import (
	"crypto/sha256"
	"crypto/sha512"

	"github.com/aisk/goblin/object"
)

func ExecuteSHA256() (object.Object, error) {
	return &object.Module{Name: "sha256", Members: map[string]object.Object{
		"sum256": &object.Function{Name: "sum256", Fn: sha256Sum256},
		"sum224": &object.Function{Name: "sum224", Fn: sha256Sum224},
	}}, nil
}

func ExecuteSHA512() (object.Object, error) {
	return &object.Module{Name: "sha512", Members: map[string]object.Object{
		"sum512":     &object.Function{Name: "sum512", Fn: sha512Sum512},
		"sum384":     &object.Function{Name: "sum384", Fn: sha512Sum384},
		"sum512_224": &object.Function{Name: "sum512_224", Fn: sha512Sum512224},
		"sum512_256": &object.Function{Name: "sum512_256", Fn: sha512Sum512256},
	}}, nil
}

func shaData(name string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(name, args)
	value := p.Any("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case object.Bytes:
		return []byte(v), nil
	case object.String:
		return []byte(v), nil
	default:
		return nil, object.NewTypeError("%s() argument 'data' must be Bytes or str, got %T", name, value)
	}
}

func sha256Sum256(args object.CallArgs) (object.Object, error) {
	data, err := shaData("sum256", args)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	return object.NewBytes(sum[:]), nil
}

func sha256Sum224(args object.CallArgs) (object.Object, error) {
	data, err := shaData("sum224", args)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum224(data)
	return object.NewBytes(sum[:]), nil
}

func sha512Sum512(args object.CallArgs) (object.Object, error) {
	data, err := shaData("sum512", args)
	if err != nil {
		return nil, err
	}
	sum := sha512.Sum512(data)
	return object.NewBytes(sum[:]), nil
}

func sha512Sum384(args object.CallArgs) (object.Object, error) {
	data, err := shaData("sum384", args)
	if err != nil {
		return nil, err
	}
	sum := sha512.Sum384(data)
	return object.NewBytes(sum[:]), nil
}

func sha512Sum512224(args object.CallArgs) (object.Object, error) {
	data, err := shaData("sum512_224", args)
	if err != nil {
		return nil, err
	}
	sum := sha512.Sum512_224(data)
	return object.NewBytes(sum[:]), nil
}

func sha512Sum512256(args object.CallArgs) (object.Object, error) {
	data, err := shaData("sum512_256", args)
	if err != nil {
		return nil, err
	}
	sum := sha512.Sum512_256(data)
	return object.NewBytes(sum[:]), nil
}
