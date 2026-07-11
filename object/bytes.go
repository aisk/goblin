package object

import (
	"bytes"
	"strconv"
	"unicode/utf8"
)

// Bytes is an immutable sequence of raw bytes. Indexing and iteration expose
// byte values as Integers in the range 0..255.
type Bytes []byte

var _ Object = Bytes{}

func NewBytes(data []byte) Bytes { return Bytes(append([]byte(nil), data...)) }

func (b Bytes) String() string { return "b" + strconv.Quote(string(b)) }
func (b Bytes) Bool() bool     { return len(b) != 0 }

func (b Bytes) Compare(other Object) (int, error) {
	v, ok := other.(Bytes)
	if !ok {
		return 0, NewTypeError("cannot compare Bytes and %T", other)
	}
	return bytes.Compare(b, v), nil
}

func (b Bytes) Add(other Object) (Object, error) {
	v, ok := other.(Bytes)
	if !ok {
		return nil, NewTypeError("cannot add Bytes and %T", other)
	}
	result := make([]byte, 0, len(b)+len(v))
	result = append(result, b...)
	result = append(result, v...)
	return Bytes(result), nil
}

func (b Bytes) Minus(Object) (Object, error)     { return nil, NewTypeError("cannot subtract from Bytes") }
func (b Bytes) Multiply(Object) (Object, error)  { return nil, NewTypeError("cannot multiply Bytes") }
func (b Bytes) Divide(Object) (Object, error)    { return nil, NewTypeError("cannot divide Bytes") }
func (b Bytes) And(other Object) (Object, error) { return Bool(b.Bool() && other.Bool()), nil }
func (b Bytes) Or(other Object) (Object, error)  { return Bool(b.Bool() || other.Bool()), nil }
func (b Bytes) Not() (Object, error)             { return Bool(!b.Bool()), nil }

func (b Bytes) Iter() ([]Object, error) {
	result := make([]Object, len(b))
	for i, value := range b {
		result[i] = Integer(value)
	}
	return result, nil
}

func (b Bytes) Index(index Object) (Object, error) {
	i, ok := index.(Integer)
	if !ok {
		return nil, NewTypeError("Bytes index must be an integer, got %T", index)
	}
	pos, err := listIndex("Bytes", i, len(b))
	if err != nil {
		return nil, err
	}
	return Integer(b[pos]), nil
}

func (b Bytes) Size(args CallArgs) (Object, error) {
	if err := requireNoArgs("size", args); err != nil {
		return nil, err
	}
	return Integer(len(b)), nil
}

func (b Bytes) Decode(args CallArgs) (Object, error) {
	if err := requireNoArgs("decode", args); err != nil {
		return nil, err
	}
	if !utf8.Valid(b) {
		return nil, NewValueError("decode() received invalid UTF-8")
	}
	return String(b), nil
}

func (b Bytes) GetAttr(name string) (Object, error) {
	switch name {
	case "size":
		return &Function{Name: name, Fn: b.Size}, nil
	case "decode":
		return &Function{Name: name, Fn: b.Decode}, nil
	case "constructor":
		return BytesConstructorFn, nil
	default:
		return nil, NewAttributeError("Bytes has no attribute '%s'", name)
	}
}

var BytesConstructorFn = &Function{Name: "Bytes", Fn: BytesConstructor}

func BytesConstructor(args CallArgs) (Object, error) {
	ap := NewArgParser("Bytes", args)
	value := ap.AnyOr("value", String(""))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case Bytes:
		return NewBytes(v), nil
	case String:
		return NewBytes([]byte(v)), nil
	case *List:
		result := make([]byte, len(v.Elements))
		for i, elem := range v.Elements {
			n, ok := elem.(Integer)
			if !ok || n < 0 || n > 255 {
				return nil, NewValueError("Bytes() element %d must be an integer from 0 to 255", i)
			}
			result[i] = byte(n)
		}
		return Bytes(result), nil
	default:
		return nil, NewTypeError("Bytes() argument 'value' must be a string, Bytes, or List, got %T", value)
	}
}
