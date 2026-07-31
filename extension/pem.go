package extension

import (
	"bytes"
	"encoding/pem"
	"sort"

	"github.com/aisk/goblin/object"
)

var pemBlockType = object.NewNativeConstructor("Block", pemBlockConstructor)

func ExecutePEM() (object.Object, error) {
	return &object.Module{Name: "pem", Members: map[string]object.Object{
		"Block":  pemBlockType.Function,
		"decode": &object.Function{Name: "decode", Fn: pemDecode},
	}}, nil
}

type PEMBlock struct {
	object.NoReflectedOps
	object.NoAssignment
	typeName string
	data     []byte
	headers  map[string]string
}

func pemHeaders(fnName string, dict *object.Dict) (map[string]string, error) {
	headers := make(map[string]string, dict.Len())
	for _, entry := range dict.Entries() {
		key, keyOK := entry.Key.(object.String)
		value, valueOK := entry.Value.(object.String)
		if !keyOK || !valueOK {
			return nil, object.NewTypeError("%s() argument 'headers' must contain str keys and values", fnName)
		}
		headers[string(key)] = string(value)
	}
	return headers, nil
}

func pemBlockConstructor(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Block", args)
	typeName := p.Str("label")
	dataObj := p.Any("data")
	headersDict := p.DictOr("headers", object.NewDict())
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if typeName == "" {
		return nil, object.NewValueError("Block() argument 'label' must not be empty")
	}
	data, err := bytesOrString("Block", "data", dataObj)
	if err != nil {
		return nil, err
	}
	headers, err := pemHeaders("Block", headersDict)
	if err != nil {
		return nil, err
	}
	return &PEMBlock{typeName: string(typeName), data: append([]byte(nil), data...), headers: headers}, nil
}

func newPEMBlock(block *pem.Block) *PEMBlock {
	return &PEMBlock{typeName: block.Type, data: append([]byte(nil), block.Bytes...), headers: block.Headers}
}

func (b *PEMBlock) native() *pem.Block {
	headers := make(map[string]string, len(b.headers))
	for key, value := range b.headers {
		headers[key] = value
	}
	return &pem.Block{Type: b.typeName, Bytes: append([]byte(nil), b.data...), Headers: headers}
}

func (b *PEMBlock) TypeName() string            { return "Block" }
func (b *PEMBlock) String() string              { return "<pem.Block " + b.typeName + ">" }
func (b *PEMBlock) ToString() (string, error)   { return b.String(), nil }
func (b *PEMBlock) ToBool() (bool, error)       { return true, nil }
func (b *PEMBlock) Not() (object.Object, error) { return object.False, nil }
func (b *PEMBlock) Equals(other object.Object) (bool, error) {
	value, ok := other.(*PEMBlock)
	if !ok || b.typeName != value.typeName || !bytes.Equal(b.data, value.data) || len(b.headers) != len(value.headers) {
		return false, nil
	}
	for key, item := range b.headers {
		if value.headers[key] != item {
			return false, nil
		}
	}
	return true, nil
}
func (b *PEMBlock) Compare(object.Object) (int, error) {
	return 0, object.NewTypeError("Block values are not ordered")
}
func (b *PEMBlock) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Block does not support addition")
}
func (b *PEMBlock) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Block does not support subtraction")
}
func (b *PEMBlock) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Block does not support multiplication")
}
func (b *PEMBlock) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Block does not support division")
}
func (b *PEMBlock) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("Block does not support iteration")
}
func (b *PEMBlock) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Block is not indexable")
}

func (b *PEMBlock) encode(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("encode", args).Finish(); err != nil {
		return nil, err
	}
	return object.NewBytes(pem.EncodeToMemory(b.native())), nil
}

func (b *PEMBlock) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(b), nil
	case "label":
		return object.String(b.typeName), nil
	case "data":
		return object.NewBytes(b.data), nil
	case "headers":
		dict := object.NewDict()
		keys := make([]string, 0, len(b.headers))
		for key := range b.headers {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			_ = dict.Set(object.String(key), object.String(b.headers[key]))
		}
		return dict, nil
	case "encode":
		return &object.Function{Name: "encode", Fn: b.encode}, nil
	default:
		return nil, object.NewAttributeError("Block has no attribute '%s'", name)
	}
}

func (b *PEMBlock) Attributes() []string {
	return []string{"attributes", "label", "data", "headers", "encode"}
}

func pemDecode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("decode", args)
	dataObj := p.Any("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	data, err := bytesOrString("decode", "data", dataObj)
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode(data)
	blockObj := object.Object(object.Nil)
	if block != nil {
		blockObj = newPEMBlock(block)
	}
	return &object.List{Elements: []object.Object{blockObj, object.NewBytes(rest)}}, nil
}

var _ object.Object = (*PEMBlock)(nil)
