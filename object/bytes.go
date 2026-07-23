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

func (b Bytes) String() string            { return "b" + strconv.Quote(string(b)) }
func (b Bytes) ToString() (string, error) { return b.String(), nil }
func (b Bytes) Bool() bool                { return len(b) != 0 }
func (b Bytes) ToBool() (bool, error)     { return b.Bool(), nil }

func (b Bytes) Equals(other Object) bool {
	v, ok := other.(Bytes)
	return ok && bytes.Equal(b, v)
}

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

func (b Bytes) Minus(Object) (Object, error)    { return nil, NewTypeError("cannot subtract from Bytes") }
func (b Bytes) Multiply(Object) (Object, error) { return nil, NewTypeError("cannot multiply Bytes") }
func (b Bytes) Divide(Object) (Object, error)   { return nil, NewTypeError("cannot divide Bytes") }
func (b Bytes) Not() (Object, error)            { return Bool(!b.Bool()), nil }

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

func bytesArg(name, param string, value Object) ([]byte, error) {
	switch v := value.(type) {
	case Bytes:
		return []byte(v), nil
	case String:
		return []byte(v), nil
	default:
		return nil, NewTypeError("%s() argument '%s' must be Bytes or str, got %T", name, param, value)
	}
}

func parseBytesArg(name, param string, args CallArgs) ([]byte, error) {
	ap := NewArgParser(name, args)
	value := ap.Any(param)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return bytesArg(name, param, value)
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
		return nil, NewTypeError("join() argument 'iterable' must be a List, got %T", iterable)
	}
	parts := make([][]byte, len(list.Elements))
	for i, elem := range list.Elements {
		part, err := bytesArg("join", "iterable", elem)
		if err != nil {
			return nil, NewTypeError("join() element %d must be Bytes or str, got %T", i, elem)
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
	oldObj, newObj := ap.Any("old"), ap.Any("new")
	count := ap.IntOr("count", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	old, err := bytesArg("replace", "old", oldObj)
	if err != nil {
		return nil, err
	}
	newValue, err := bytesArg("replace", "new", newObj)
	if err != nil {
		return nil, err
	}
	return NewBytes(bytes.Replace(b, old, newValue, int(count))), nil
}

func (b Bytes) Split(args CallArgs) (Object, error) {
	ap := NewArgParser("split", args)
	sepObj := ap.Any("sep")
	count := ap.IntOr("count", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	sep, err := bytesArg("split", "sep", sepObj)
	if err != nil {
		return nil, err
	}
	return bytesList(bytes.SplitN(b, sep, int(count))), nil
}

func (b Bytes) SplitAfter(args CallArgs) (Object, error) {
	ap := NewArgParser("split_after", args)
	sepObj := ap.Any("sep")
	count := ap.IntOr("count", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	sep, err := bytesArg("split_after", "sep", sepObj)
	if err != nil {
		return nil, err
	}
	return bytesList(bytes.SplitAfterN(b, sep, int(count))), nil
}

func (b Bytes) Fields(args CallArgs) (Object, error) {
	if err := requireNoArgs("fields", args); err != nil {
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
	if err := requireNoArgs("upper", args); err != nil {
		return nil, err
	}
	return NewBytes(bytes.ToUpper(b)), nil
}
func (b Bytes) Lower(args CallArgs) (Object, error) {
	if err := requireNoArgs("lower", args); err != nil {
		return nil, err
	}
	return NewBytes(bytes.ToLower(b)), nil
}
func (b Bytes) Title(args CallArgs) (Object, error) {
	if err := requireNoArgs("title", args); err != nil {
		return nil, err
	}
	return NewBytes(bytes.Title(b)), nil
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
	case "attributes":
		return AttributesFunction(b), nil
	case "size":
		return &Function{Name: name, Fn: b.Size}, nil
	case "decode":
		return &Function{Name: name, Fn: b.Decode}, nil
	case "contains":
		return &Function{Name: name, Fn: b.Contains}, nil
	case "contains_any":
		return &Function{Name: name, Fn: b.ContainsAny}, nil
	case "contains_rune":
		return &Function{Name: name, Fn: b.ContainsRune}, nil
	case "count":
		return &Function{Name: name, Fn: b.Count}, nil
	case "equal_fold":
		return &Function{Name: name, Fn: b.EqualFold}, nil
	case "compare":
		return &Function{Name: name, Fn: b.CompareBytes}, nil
	case "has_prefix":
		return &Function{Name: name, Fn: b.HasPrefix}, nil
	case "has_suffix":
		return &Function{Name: name, Fn: b.HasSuffix}, nil
	case "index":
		return &Function{Name: name, Fn: b.IndexOf}, nil
	case "last_index":
		return &Function{Name: name, Fn: b.LastIndex}, nil
	case "index_any":
		return &Function{Name: name, Fn: b.IndexAny}, nil
	case "last_index_any":
		return &Function{Name: name, Fn: b.LastIndexAny}, nil
	case "index_byte":
		return &Function{Name: name, Fn: b.IndexByte}, nil
	case "last_index_byte":
		return &Function{Name: name, Fn: b.LastIndexByte}, nil
	case "index_rune":
		return &Function{Name: name, Fn: b.IndexRune}, nil
	case "join":
		return &Function{Name: name, Fn: b.Join}, nil
	case "repeat":
		return &Function{Name: name, Fn: b.Repeat}, nil
	case "replace":
		return &Function{Name: name, Fn: b.Replace}, nil
	case "split":
		return &Function{Name: name, Fn: b.Split}, nil
	case "split_after":
		return &Function{Name: name, Fn: b.SplitAfter}, nil
	case "fields":
		return &Function{Name: name, Fn: b.Fields}, nil
	case "cut":
		return &Function{Name: name, Fn: b.Cut}, nil
	case "cut_prefix":
		return &Function{Name: name, Fn: b.CutPrefix}, nil
	case "cut_suffix":
		return &Function{Name: name, Fn: b.CutSuffix}, nil
	case "trim":
		return &Function{Name: name, Fn: b.Trim}, nil
	case "trim_left":
		return &Function{Name: name, Fn: b.TrimLeft}, nil
	case "trim_right":
		return &Function{Name: name, Fn: b.TrimRight}, nil
	case "trim_prefix":
		return &Function{Name: name, Fn: b.TrimPrefix}, nil
	case "trim_suffix":
		return &Function{Name: name, Fn: b.TrimSuffix}, nil
	case "upper":
		return &Function{Name: name, Fn: b.Upper}, nil
	case "lower":
		return &Function{Name: name, Fn: b.Lower}, nil
	case "title":
		return &Function{Name: name, Fn: b.Title}, nil
	case "to_valid_utf8":
		return &Function{Name: name, Fn: b.ToValidUTF8}, nil
	case "constructor":
		return BytesConstructorFn, nil
	default:
		return nil, NewAttributeError("Bytes has no attribute '%s'", name)
	}
}

func (b Bytes) Attributes() []string {
	return []string{
		"attributes", "size", "decode", "contains", "contains_any", "contains_rune", "count",
		"equal_fold", "compare", "has_prefix", "has_suffix", "index", "last_index",
		"index_any", "last_index_any", "index_byte", "last_index_byte", "index_rune",
		"join", "repeat", "replace", "split", "split_after", "fields", "cut", "cut_prefix",
		"cut_suffix", "trim", "trim_left", "trim_right", "trim_prefix", "trim_suffix",
		"upper", "lower", "title", "to_valid_utf8", "constructor",
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
