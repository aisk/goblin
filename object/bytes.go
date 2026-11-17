package object

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"unicode/utf8"
)

// Bytes is an immutable sequence of raw bytes. Indexing and iteration expose
// byte values as Integers in the range 0..255.
type Bytes []byte

var _ Object = Bytes{}

func NewBytes(data []byte) Bytes { return Bytes(append([]byte(nil), data...)) }

func (b Bytes) TypeName() string { return "Bytes" }

func (b Bytes) String() string            { return "b" + strconv.Quote(string(b)) }
func (b Bytes) ToString() (string, error) { return b.String(), nil }
func (b Bytes) ToBool() (bool, error)     { return len(b) != 0, nil }

func (b Bytes) Equals(other Object) (bool, error) {
	v, ok := other.(Bytes)
	return ok && bytes.Equal(b, v), nil
}

func (b Bytes) Compare(other Object) (int, error) {
	v, ok := other.(Bytes)
	if !ok {
		return 0, NewOperandError(OpCompare, "Bytes", other)
	}
	return bytes.Compare(b, v), nil
}

func (b Bytes) Add(other Object) (Object, error) {
	v, ok := other.(Bytes)
	if !ok {
		return nil, NewOperandError(OpAdd, "Bytes", other)
	}
	result := make([]byte, 0, len(b)+len(v))
	result = append(result, b...)
	result = append(result, v...)
	return Bytes(result), nil
}

func (b Bytes) Minus(Object) (Object, error) {
	return nil, Unsupported("Bytes", ErrFmtCannotSubtract)
}
func (b Bytes) Multiply(other Object) (Object, error) {
	n, ok := other.(Integer)
	if !ok {
		return nil, NewOperandError(OpMultiply, "Bytes", other)
	}
	if n < 0 {
		return nil, NewValueError("cannot multiply Bytes by negative number")
	}
	if err := checkRepeat(len(b), n); err != nil {
		return nil, err
	}
	return Bytes(bytes.Repeat(b, int(n))), nil
}

// RMultiply repeats the bytes when they are the right operand, so `3 * b`
// reads the same as `b * 3`.
func (b Bytes) RMultiply(left Object) (Object, bool, error) {
	if _, ok := left.(Integer); !ok {
		return nil, false, nil
	}
	result, err := b.Multiply(left)
	return result, true, err
}
func (b Bytes) Divide(Object) (Object, error) {
	return nil, Unsupported("Bytes", ErrFmtCannotDivide)
}
func (b Bytes) Modulo(Object) (Object, error) {
	return nil, Unsupported("Bytes", ErrFmtCannotModulo)
}

// A named slice type cannot embed NoAssignment and NoReflectedOps, so Bytes
// declines assignment and the reflected operators other than repetition
// itself.
func (b Bytes) SetIndex(Object, Object) (bool, error) { return false, nil }
func (b Bytes) SetAttr(string, Object) (bool, error)  { return false, nil }
func (b Bytes) RAdd(Object) (Object, bool, error)     { return nil, false, nil }
func (b Bytes) RMinus(Object) (Object, bool, error)   { return nil, false, nil }
func (b Bytes) RDivide(Object) (Object, bool, error)  { return nil, false, nil }
func (b Bytes) RModulo(Object) (Object, bool, error)  { return nil, false, nil }

// Iter returns the bytes of b as Integers.
func (b Bytes) Iter() ([]Object, error) { return Collect(b) }

// Iterator walks b byte by byte. Bytes are immutable, so the pass reads them
// lazily without changing what it visits.
func (b Bytes) Iterator() (Iterator, error) {
	i := 0
	return IteratorFunc(func() (Object, bool, error) {
		if i >= len(b) {
			return nil, false, nil
		}
		v := Integer(b[i])
		i++
		return v, true, nil
	}), nil
}

func (b Bytes) Index(index Object) (Object, error) {
	idx, err := indexArg("Bytes", index)
	if err != nil {
		return nil, err
	}
	i, err := ResolveIndex("Bytes", idx, len(b))
	if err != nil {
		return nil, err
	}
	return Integer(b[i]), nil
}

// Size returns the length in bytes, exposed to Goblin as the read-only
// property bytes.size.
func (b Bytes) Size() Integer {
	return Integer(len(b))
}

func parseBytesArg(name, param string, args CallArgs) ([]byte, error) {
	ap := NewArgParser(name, args)
	value := ap.BytesLike(param)
	return value, ap.Finish()
}

func (b Bytes) Contains(args CallArgs) (Object, error) {
	value, err := parseBytesArg("contains", "value", args)
	if err != nil {
		return nil, err
	}
	return bytesSeq(b).contains(string(value)), nil
}

func (b Bytes) Count(args CallArgs) (Object, error) {
	value, err := parseBytesArg("count", "value", args)
	if err != nil {
		return nil, err
	}
	return bytesSeq(b).count(string(value)), nil
}

// IndexOf returns the byte offset of the first occurrence of value at or
// after start, or nil.
func (b Bytes) IndexOf(args CallArgs) (Object, error) {
	ap := NewArgParser("index", args)
	value := ap.BytesLike("value")
	start := ap.IntOr("start", 0)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return bytesSeq(b).index(string(value), start)
}

func (b Bytes) HasPrefix(args CallArgs) (Object, error) {
	prefix, err := parseBytesArg("has_prefix", "prefix", args)
	if err != nil {
		return nil, err
	}
	return Bool(bytes.HasPrefix(b, prefix)), nil
}

func (b Bytes) HasSuffix(args CallArgs) (Object, error) {
	suffix, err := parseBytesArg("has_suffix", "suffix", args)
	if err != nil {
		return nil, err
	}
	return Bool(bytes.HasSuffix(b, suffix)), nil
}

func (b Bytes) TrimPrefix(args CallArgs) (Object, error) {
	prefix, err := parseBytesArg("trim_prefix", "prefix", args)
	if err != nil {
		return nil, err
	}
	return NewBytes(bytes.TrimPrefix(b, prefix)), nil
}

func (b Bytes) TrimSuffix(args CallArgs) (Object, error) {
	suffix, err := parseBytesArg("trim_suffix", "suffix", args)
	if err != nil {
		return nil, err
	}
	return NewBytes(bytes.TrimSuffix(b, suffix)), nil
}

func (b Bytes) Replace(args CallArgs) (Object, error) {
	ap := NewArgParser("replace", args)
	old, newValue := ap.BytesLike("old"), ap.BytesLike("new")
	count, limited := ap.NullableInt("count")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return bytesSeq(b).replace(string(old), string(newValue), count, limited)
}

// Split takes a separator: splitting at whitespace is a text operation, for
// a String.
func (b Bytes) Split(args CallArgs) (Object, error) {
	ap := NewArgParser("split", args)
	sep := ap.BytesLike("sep")
	count, limited := ap.NullableInt("count")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return bytesSeq(b).split(string(sep), true, count, limited)
}

func (b Bytes) Cut(args CallArgs) (Object, error) {
	sep, err := parseBytesArg("cut", "sep", args)
	if err != nil {
		return nil, err
	}
	return bytesSeq(b).cut(string(sep)), nil
}

func (b Bytes) Decode(args CallArgs) (Object, error) {
	if err := RequireNoArgs("decode", args); err != nil {
		return nil, err
	}
	if !utf8.Valid(b) {
		return nil, NewValueError("decode() invalid UTF-8")
	}
	return String(b), nil
}

// ToHex returns the bytes as two lower-case hexadecimal digits each.
func (b Bytes) ToHex(args CallArgs) (Object, error) {
	if err := RequireNoArgs("to_hex", args); err != nil {
		return nil, err
	}
	return String(hex.EncodeToString(b)), nil
}

func (b Bytes) GetAttr(name string) (Object, error) {
	switch name {
	case "size":
		return b.Size(), nil
	}
	if fn, ok := bytesMethods.bind(b, name); ok {
		return fn, nil
	}
	return nil, NewAttributeError("Bytes has no attribute '%s'", name)
}

func (b Bytes) Attributes() []string { return bytesMethods.attributes("size") }

var bytesMethods = newMethodTable(
	"decode", "to_hex", "contains", "count", "index", "has_prefix", "has_suffix",
	"trim_prefix", "trim_suffix", "replace", "split", "cut",
)

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
		return nil, NewTypeError("Bytes() argument 'value' must be String, Bytes, or List, got %s", value.TypeName())
	}
}
