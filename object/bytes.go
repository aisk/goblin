package object

import (
	"bytes"
	"strconv"
	"unicode"
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
func (b Bytes) Multiply(Object) (Object, error) {
	return nil, Unsupported("Bytes", ErrFmtCannotMultiply)
}
func (b Bytes) Divide(Object) (Object, error) {
	return nil, Unsupported("Bytes", ErrFmtCannotDivide)
}
func (b Bytes) Modulo(Object) (Object, error) {
	return nil, Unsupported("Bytes", ErrFmtCannotModulo)
}

// A named slice type cannot embed NoAssignment and NoReflectedOps, so Bytes
// declines assignment and the reflected operators itself.
func (b Bytes) SetIndex(Object, Object) (bool, error)  { return false, nil }
func (b Bytes) SetAttr(string, Object) (bool, error)   { return false, nil }
func (b Bytes) RAdd(Object) (Object, bool, error)      { return nil, false, nil }
func (b Bytes) RMinus(Object) (Object, bool, error)    { return nil, false, nil }
func (b Bytes) RMultiply(Object) (Object, bool, error) { return nil, false, nil }
func (b Bytes) RDivide(Object) (Object, bool, error)   { return nil, false, nil }
func (b Bytes) RModulo(Object) (Object, bool, error)   { return nil, false, nil }

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
		return nil, NewTypeError("Bytes index must be Integer, got %s", index.TypeName())
	}
	if i < 0 || int(i) >= len(b) {
		return nil, NewIndexError("Bytes index out of range: %d", int64(i))
	}
	return Integer(b[i]), nil
}

// Size returns the length in bytes, exposed to Goblin as the read-only
// property bytes.size.
func (b Bytes) Size() Integer {
	return Integer(len(b))
}

func bytesArg(name, param string, value Object) ([]byte, error) {
	switch v := value.(type) {
	case Bytes:
		return []byte(v), nil
	case String:
		return []byte(v), nil
	default:
		return nil, NewTypeError("%s() argument '%s' must be Bytes or String, got %s", name, param, value.TypeName())
	}
}

func parseBytesArg(name, param string, args CallArgs) ([]byte, error) {
	ap := NewArgParser(name, args)
	value := ap.BytesLike(param)
	return value, ap.Finish()
}

func bytesList(values [][]byte) Object {
	elements := make([]Object, len(values))
	for i, value := range values {
		elements[i] = NewBytes(value)
	}
	return &List{Elements: elements}
}

func (b Bytes) Contains(args CallArgs) (Object, error) {
	sub, err := parseBytesArg("contains", "sub", args)
	if err != nil {
		return nil, err
	}
	return Bool(bytes.Contains(b, sub)), nil
}

func (b Bytes) ContainsAny(args CallArgs) (Object, error) {
	chars, err := parseBytesArg("contains_any", "chars", args)
	if err != nil {
		return nil, err
	}
	return Bool(bytes.ContainsAny(b, string(chars))), nil
}

func (b Bytes) ContainsRune(args CallArgs) (Object, error) {
	ap := NewArgParser("contains_rune", args)
	r := ap.Int("rune")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if r < 0 || r > utf8.MaxRune {
		return nil, NewValueError("contains_rune() rune must be a valid Unicode code point")
	}
	return Bool(bytes.ContainsRune(b, rune(r))), nil
}

func (b Bytes) Count(args CallArgs) (Object, error) {
	sub, err := parseBytesArg("count", "sub", args)
	if err != nil {
		return nil, err
	}
	return Integer(bytes.Count(b, sub)), nil
}

func (b Bytes) EqualFold(args CallArgs) (Object, error) {
	other, err := parseBytesArg("equal_fold", "other", args)
	if err != nil {
		return nil, err
	}
	return Bool(bytes.EqualFold(b, other)), nil
}

func (b Bytes) CompareBytes(args CallArgs) (Object, error) {
	other, err := parseBytesArg("compare", "other", args)
	if err != nil {
		return nil, err
	}
	return Integer(bytes.Compare(b, other)), nil
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

func (b Bytes) IndexOf(args CallArgs) (Object, error) {
	sub, err := parseBytesArg("index", "sub", args)
	if err != nil {
		return nil, err
	}
	return Integer(bytes.Index(b, sub)), nil
}

func (b Bytes) LastIndex(args CallArgs) (Object, error) {
	sub, err := parseBytesArg("last_index", "sub", args)
	if err != nil {
		return nil, err
	}
	return Integer(bytes.LastIndex(b, sub)), nil
}

func (b Bytes) IndexAny(args CallArgs) (Object, error) {
	chars, err := parseBytesArg("index_any", "chars", args)
	if err != nil {
		return nil, err
	}
	return Integer(bytes.IndexAny(b, string(chars))), nil
}

func (b Bytes) LastIndexAny(args CallArgs) (Object, error) {
	chars, err := parseBytesArg("last_index_any", "chars", args)
	if err != nil {
		return nil, err
	}
	return Integer(bytes.LastIndexAny(b, string(chars))), nil
}

func (b Bytes) IndexByte(args CallArgs) (Object, error) {
	return b.byteIndex("index_byte", args, false)
}
func (b Bytes) LastIndexByte(args CallArgs) (Object, error) {
	return b.byteIndex("last_index_byte", args, true)
}

func (b Bytes) byteIndex(name string, args CallArgs, last bool) (Object, error) {
	ap := NewArgParser(name, args)
	value := ap.Int("value")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if value < 0 || value > 255 {
		return nil, NewValueError("%s() value must be from 0 to 255", name)
	}
	if last {
		return Integer(bytes.LastIndexByte(b, byte(value))), nil
	}
	return Integer(bytes.IndexByte(b, byte(value))), nil
}

func (b Bytes) IndexRune(args CallArgs) (Object, error) {
	ap := NewArgParser("index_rune", args)
	r := ap.Int("rune")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if r < 0 || r > utf8.MaxRune {
		return nil, NewValueError("index_rune() rune must be a valid Unicode code point")
	}
	return Integer(bytes.IndexRune(b, rune(r))), nil
}

func (b Bytes) Join(args CallArgs) (Object, error) {
	ap := NewArgParser("join", args)
	iterable := ap.Any("iterable")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	list, ok := iterable.(*List)
	if !ok {
		return nil, NewTypeError("join() argument 'iterable' must be a List, got %s", iterable.TypeName())
	}
	parts := make([][]byte, len(list.Elements))
	for i, elem := range list.Elements {
		part, err := bytesArg("join", "iterable", elem)
		if err != nil {
			return nil, NewTypeError("join() element %d must be Bytes or String, got %s", i, elem.TypeName())
		}
		parts[i] = part
	}
	return NewBytes(bytes.Join(parts, b)), nil
}

func (b Bytes) Repeat(args CallArgs) (Object, error) {
	ap := NewArgParser("repeat", args)
	count := ap.Int("count")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if count < 0 {
		return nil, NewValueError("repeat() count must not be negative")
	}
	return NewBytes(bytes.Repeat(b, int(count))), nil
}

func (b Bytes) Replace(args CallArgs) (Object, error) {
	ap := NewArgParser("replace", args)
	old, newValue := ap.BytesLike("old"), ap.BytesLike("new")
	count := ap.IntOr("count", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return NewBytes(bytes.Replace(b, old, newValue, int(count))), nil
}

func (b Bytes) Split(args CallArgs) (Object, error) {
	ap := NewArgParser("split", args)
	sep := ap.BytesLike("sep")
	count := ap.IntOr("count", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return bytesList(bytes.SplitN(b, sep, int(count))), nil
}

func (b Bytes) SplitAfter(args CallArgs) (Object, error) {
	ap := NewArgParser("split_after", args)
	sep := ap.BytesLike("sep")
	count := ap.IntOr("count", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return bytesList(bytes.SplitAfterN(b, sep, int(count))), nil
}

func (b Bytes) Fields(args CallArgs) (Object, error) {
	if err := RequireNoArgs("fields", args); err != nil {
		return nil, err
	}
	return bytesList(bytes.Fields(b)), nil
}

func (b Bytes) Cut(args CallArgs) (Object, error) {
	sep, err := parseBytesArg("cut", "sep", args)
	if err != nil {
		return nil, err
	}
	before, after, found := bytes.Cut(b, sep)
	return &List{Elements: []Object{NewBytes(before), NewBytes(after), Bool(found)}}, nil
}

func (b Bytes) CutPrefix(args CallArgs) (Object, error) {
	prefix, err := parseBytesArg("cut_prefix", "prefix", args)
	if err != nil {
		return nil, err
	}
	value, found := bytes.CutPrefix(b, prefix)
	return &List{Elements: []Object{NewBytes(value), Bool(found)}}, nil
}

func (b Bytes) CutSuffix(args CallArgs) (Object, error) {
	suffix, err := parseBytesArg("cut_suffix", "suffix", args)
	if err != nil {
		return nil, err
	}
	value, found := bytes.CutSuffix(b, suffix)
	return &List{Elements: []Object{NewBytes(value), Bool(found)}}, nil
}

func (b Bytes) Trim(args CallArgs) (Object, error)      { return b.trim("trim", args, trimBoth) }
func (b Bytes) TrimLeft(args CallArgs) (Object, error)  { return b.trim("trim_left", args, trimLeft) }
func (b Bytes) TrimRight(args CallArgs) (Object, error) { return b.trim("trim_right", args, trimRight) }

func (b Bytes) trim(name string, args CallArgs, direction trimDirection) (Object, error) {
	ap := NewArgParser(name, args)
	cutsetObj := ap.AnyOr("cutset", Nil)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if _, ok := cutsetObj.(Unit); ok {
		switch direction {
		case trimBoth:
			return NewBytes(bytes.TrimSpace(b)), nil
		case trimLeft:
			return NewBytes(bytes.TrimLeftFunc(b, unicode.IsSpace)), nil
		case trimRight:
			return NewBytes(bytes.TrimRightFunc(b, unicode.IsSpace)), nil
		}
	}
	cutset, err := bytesArg(name, "cutset", cutsetObj)
	if err != nil {
		return nil, err
	}
	switch direction {
	case trimBoth:
		return NewBytes(bytes.Trim(b, string(cutset))), nil
	case trimLeft:
		return NewBytes(bytes.TrimLeft(b, string(cutset))), nil
	default:
		return NewBytes(bytes.TrimRight(b, string(cutset))), nil
	}
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

func (b Bytes) Upper(args CallArgs) (Object, error) {
	if err := RequireNoArgs("upper", args); err != nil {
		return nil, err
	}
	return NewBytes(bytes.ToUpper(b)), nil
}
func (b Bytes) Lower(args CallArgs) (Object, error) {
	if err := RequireNoArgs("lower", args); err != nil {
		return nil, err
	}
	return NewBytes(bytes.ToLower(b)), nil
}
func (b Bytes) Title(args CallArgs) (Object, error) {
	if err := RequireNoArgs("title", args); err != nil {
		return nil, err
	}
	return NewBytes(bytes.Title(b)), nil
}

func (b Bytes) ToTitle(args CallArgs) (Object, error) {
	if err := RequireNoArgs("to_title", args); err != nil {
		return nil, err
	}
	return NewBytes(bytes.ToTitle(b)), nil
}

func (b Bytes) ToValidUTF8(args CallArgs) (Object, error) {
	ap := NewArgParser("to_valid_utf8", args)
	replacementObj := ap.AnyOr("replacement", Bytes{})
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	replacement, err := bytesArg("to_valid_utf8", "replacement", replacementObj)
	if err != nil {
		return nil, err
	}
	return NewBytes(bytes.ToValidUTF8(b, replacement)), nil
}

func (b Bytes) Decode(args CallArgs) (Object, error) {
	if err := RequireNoArgs("decode", args); err != nil {
		return nil, err
	}
	if !utf8.Valid(b) {
		return nil, NewValueError("decode() received invalid UTF-8")
	}
	return String(b), nil
}

func (b Bytes) GetAttr(name string) (Object, error) {
	switch name {
	case "attributes":
		return AttributesFunction(b), nil
	case "size":
		return b.Size(), nil
	case "constructor":
		return BytesConstructorFn, nil
	}
	if fn, ok := bytesMethods.bind(b, name); ok {
		return fn, nil
	}
	return nil, NewAttributeError("Bytes has no attribute '%s'", name)
}

func (b Bytes) Attributes() []string { return bytesMethods.attributes("size") }

var bytesMethods = newMethodTable(
	"decode", "contains", "contains_any", "contains_rune", "count", "equal_fold",
	"compare", "has_prefix", "has_suffix", "index", "last_index", "index_any",
	"last_index_any", "index_byte", "last_index_byte", "index_rune", "join",
	"repeat", "replace", "split", "split_after", "fields", "cut", "cut_prefix",
	"cut_suffix", "trim", "trim_left", "trim_right", "trim_prefix", "trim_suffix",
	"upper", "lower", "title", "to_title", "to_valid_utf8",
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
