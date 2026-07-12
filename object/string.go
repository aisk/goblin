package object

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type String string

var _ Object = String("")

type trimDirection uint8

const (
	trimBoth trimDirection = iota
	trimLeft
	trimRight
)

func (s String) Size(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("size", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("size() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	return Integer(len([]rune(string(s)))), nil
}

func (s String) Upper(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("upper", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("upper() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	return String(strings.ToUpper(string(s))), nil
}

func (s String) Lower(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("lower", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("lower() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	return String(strings.ToLower(string(s))), nil
}

func (s String) HasPrefix(args CallArgs) (Object, error) {
	ap := NewArgParser("has_prefix", args)
	prefix := ap.Str("prefix")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return Bool(strings.HasPrefix(string(s), string(prefix))), nil
}

func (s String) HasSuffix(args CallArgs) (Object, error) {
	ap := NewArgParser("has_suffix", args)
	suffix := ap.Str("suffix")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return Bool(strings.HasSuffix(string(s), string(suffix))), nil
}

func (s String) Trim(args CallArgs) (Object, error) {
	ap := NewArgParser("trim", args)
	cutset := ap.AnyOr("cutset", Nil)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return s.trimWith("trim", cutset, trimBoth)
}

func (s String) trimWith(name string, cutset Object, direction trimDirection) (Object, error) {
	value := string(s)
	if _, space := cutset.(Unit); space {
		switch direction {
		case trimBoth:
			return String(strings.TrimSpace(value)), nil
		case trimLeft:
			return String(strings.TrimLeftFunc(value, unicode.IsSpace)), nil
		case trimRight:
			return String(strings.TrimRightFunc(value, unicode.IsSpace)), nil
		}
	}
	chars, ok := cutset.(String)
	if !ok {
		return nil, NewTypeError("%s() argument 'cutset' must be str or none, got %T", name, cutset)
	}
	switch direction {
	case trimBoth:
		return String(strings.Trim(value, string(chars))), nil
	case trimLeft:
		return String(strings.TrimLeft(value, string(chars))), nil
	case trimRight:
		return String(strings.TrimRight(value, string(chars))), nil
	}
	panic("invalid trim direction")
}

func (s String) Contains(args CallArgs) (Object, error) {
	ap := NewArgParser("contains", args)
	substr := ap.Str("substr")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return Bool(strings.Contains(string(s), string(substr))), nil
}

func (s String) ContainsAny(args CallArgs) (Object, error) {
	ap := NewArgParser("contains_any", args)
	chars := ap.Str("chars")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return Bool(strings.ContainsAny(string(s), string(chars))), nil
}

func (s String) Count(args CallArgs) (Object, error) {
	ap := NewArgParser("count", args)
	substr := ap.Str("substr")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return Integer(strings.Count(string(s), string(substr))), nil
}

func (s String) EqualFold(args CallArgs) (Object, error) {
	ap := NewArgParser("equal_fold", args)
	other := ap.Str("other")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return Bool(strings.EqualFold(string(s), string(other))), nil
}

func (s String) CompareText(args CallArgs) (Object, error) {
	ap := NewArgParser("compare", args)
	other := ap.Str("other")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return Integer(strings.Compare(string(s), string(other))), nil
}

func (s String) IndexOf(args CallArgs) (Object, error) {
	ap := NewArgParser("index", args)
	substr := ap.Str("substr")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return runeIndex(string(s), strings.Index(string(s), string(substr))), nil
}

func (s String) LastIndex(args CallArgs) (Object, error) {
	ap := NewArgParser("last_index", args)
	substr := ap.Str("substr")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return runeIndex(string(s), strings.LastIndex(string(s), string(substr))), nil
}

func (s String) IndexAny(args CallArgs) (Object, error) {
	ap := NewArgParser("index_any", args)
	chars := ap.Str("chars")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return runeIndex(string(s), strings.IndexAny(string(s), string(chars))), nil
}

func (s String) LastIndexAny(args CallArgs) (Object, error) {
	ap := NewArgParser("last_index_any", args)
	chars := ap.Str("chars")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return runeIndex(string(s), strings.LastIndexAny(string(s), string(chars))), nil
}

// Go's strings indexes are byte offsets. Goblin strings iterate and size in
// Unicode code points, so string indexes consistently use rune offsets.
func runeIndex(s string, byteIndex int) Integer {
	if byteIndex < 0 {
		return -1
	}
	return Integer(utf8.RuneCountInString(s[:byteIndex]))
}

func (s String) Repeat(args CallArgs) (Object, error) {
	ap := NewArgParser("repeat", args)
	count := ap.Int("count")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if count < 0 {
		return nil, NewValueError("repeat() count must not be negative")
	}
	return String(strings.Repeat(string(s), int(count))), nil
}

// Replace combines strings.Replace and strings.ReplaceAll. count defaults to
// -1, meaning all matches, and may be supplied by name.
func (s String) Replace(args CallArgs) (Object, error) {
	ap := NewArgParser("replace", args)
	old, newValue := ap.Str("old"), ap.Str("new")
	count := ap.IntOr("count", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return String(strings.Replace(string(s), string(old), string(newValue), int(count))), nil
}

func stringsList(values []string) Object {
	elements := make([]Object, len(values))
	for i, value := range values {
		elements[i] = String(value)
	}
	return &List{Elements: elements}
}

// Split combines strings.Split and strings.SplitN. count=-1 keeps all pieces.
func (s String) Split(args CallArgs) (Object, error) {
	ap := NewArgParser("split", args)
	sep := ap.Str("sep")
	count := ap.IntOr("count", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return stringsList(strings.SplitN(string(s), string(sep), int(count))), nil
}

// SplitAfter combines strings.SplitAfter and strings.SplitAfterN.
func (s String) SplitAfter(args CallArgs) (Object, error) {
	ap := NewArgParser("split_after", args)
	sep := ap.Str("sep")
	count := ap.IntOr("count", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return stringsList(strings.SplitAfterN(string(s), string(sep), int(count))), nil
}

func (s String) Fields(args CallArgs) (Object, error) {
	if err := noStringArgs("fields", args); err != nil {
		return nil, err
	}
	return stringsList(strings.Fields(string(s))), nil
}

func (s String) Title(args CallArgs) (Object, error) {
	if err := noStringArgs("title", args); err != nil {
		return nil, err
	}
	return String(strings.Title(string(s))), nil //nolint:staticcheck -- mirrors strings.Title
}

func (s String) ToTitle(args CallArgs) (Object, error) {
	if err := noStringArgs("to_title", args); err != nil {
		return nil, err
	}
	return String(strings.ToTitle(string(s))), nil
}

func (s String) ToValidUTF8(args CallArgs) (Object, error) {
	ap := NewArgParser("to_valid_utf8", args)
	replacement := ap.StrOr("replacement", "")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return String(strings.ToValidUTF8(string(s), string(replacement))), nil
}

func (s String) TrimLeft(args CallArgs) (Object, error) {
	ap := NewArgParser("trim_left", args)
	cutset := ap.AnyOr("cutset", Nil)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return s.trimWith("trim_left", cutset, trimLeft)
}

func (s String) TrimRight(args CallArgs) (Object, error) {
	ap := NewArgParser("trim_right", args)
	cutset := ap.AnyOr("cutset", Nil)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return s.trimWith("trim_right", cutset, trimRight)
}

func (s String) TrimPrefix(args CallArgs) (Object, error) {
	ap := NewArgParser("trim_prefix", args)
	prefix := ap.Str("prefix")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return String(strings.TrimPrefix(string(s), string(prefix))), nil
}

func (s String) TrimSuffix(args CallArgs) (Object, error) {
	ap := NewArgParser("trim_suffix", args)
	suffix := ap.Str("suffix")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return String(strings.TrimSuffix(string(s), string(suffix))), nil
}

func (s String) Cut(args CallArgs) (Object, error) {
	ap := NewArgParser("cut", args)
	sep := ap.Str("sep")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	before, after, found := strings.Cut(string(s), string(sep))
	return &List{Elements: []Object{String(before), String(after), Bool(found)}}, nil
}

func (s String) CutPrefix(args CallArgs) (Object, error) {
	ap := NewArgParser("cut_prefix", args)
	prefix := ap.Str("prefix")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	value, found := strings.CutPrefix(string(s), string(prefix))
	return &List{Elements: []Object{String(value), Bool(found)}}, nil
}

func (s String) CutSuffix(args CallArgs) (Object, error) {
	ap := NewArgParser("cut_suffix", args)
	suffix := ap.Str("suffix")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	value, found := strings.CutSuffix(string(s), string(suffix))
	return &List{Elements: []Object{String(value), Bool(found)}}, nil
}

func noStringArgs(name string, args CallArgs) error {
	if err := RequireNoKeyword(name, args); err != nil {
		return err
	}
	if len(args.Positional) != 0 {
		return NewTypeError("%s() takes exactly 0 arguments, got %d", name, len(args.Positional))
	}
	return nil
}

func (s String) String() string {
	return string(s)
}

// Literal returns the quoted Goblin source representation of the string.
func (s String) Literal() string {
	return strconv.Quote(string(s))
}

func (s String) Bool() bool {
	if s == "" {
		return false
	}
	return true
}

func (s String) Compare(other Object) (int, error) {
	switch v := other.(type) {
	case String:
		a, b := string(s), string(v)
		if a < b {
			return -1, nil
		}
		if a > b {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, NewTypeError("cannot compare String and %T", other)
	}
}

func (s String) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case String:
		return String(string(s) + string(v)), nil
	case Integer:
		return String(string(s) + v.String()), nil
	case Bool:
		return String(string(s) + v.String()), nil
	default:
		return nil, NewTypeError("cannot add String and %T", other)
	}
}

func (s String) Minus(other Object) (Object, error) {
	return nil, NewTypeError("cannot subtract from String")
}

func (s String) Multiply(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		result := ""
		for i := int64(0); i < int64(v); i++ {
			result += string(s)
		}
		return String(result), nil
	default:
		return nil, NewTypeError("cannot multiply String and %T", other)
	}
}

func (s String) Divide(other Object) (Object, error) {
	return nil, NewTypeError("cannot divide String")
}

func (s String) And(other Object) (Object, error) {
	return Bool(s.Bool() && other.Bool()), nil
}

func (s String) Or(other Object) (Object, error) {
	return Bool(s.Bool() || other.Bool()), nil
}

func (s String) Not() (Object, error) {
	return Bool(!s.Bool()), nil
}

func (s String) Iter() ([]Object, error) {
	// String can be iterated character by character
	var result []Object
	for _, char := range string(s) {
		result = append(result, String(string(char)))
	}
	return result, nil
}

func (s String) Index(index Object) (Object, error) {
	return nil, NewTypeError("String is not indexable")
}

func (s String) GetAttr(name string) (Object, error) {
	switch name {
	case "attributes":
		return AttributesFunction(s), nil
	case "size":
		return &Function{Name: "size", Fn: s.Size}, nil
	case "upper":
		return &Function{Name: "upper", Fn: s.Upper}, nil
	case "lower":
		return &Function{Name: "lower", Fn: s.Lower}, nil
	case "has_prefix":
		return &Function{Name: "has_prefix", Fn: s.HasPrefix}, nil
	case "has_suffix":
		return &Function{Name: "has_suffix", Fn: s.HasSuffix}, nil
	case "trim":
		return &Function{Name: "trim", Fn: s.Trim}, nil
	case "contains":
		return &Function{Name: "contains", Fn: s.Contains}, nil
	case "contains_any":
		return &Function{Name: name, Fn: s.ContainsAny}, nil
	case "count":
		return &Function{Name: name, Fn: s.Count}, nil
	case "equal_fold":
		return &Function{Name: name, Fn: s.EqualFold}, nil
	case "compare":
		return &Function{Name: name, Fn: s.CompareText}, nil
	case "index":
		return &Function{Name: name, Fn: s.IndexOf}, nil
	case "last_index":
		return &Function{Name: name, Fn: s.LastIndex}, nil
	case "index_any":
		return &Function{Name: name, Fn: s.IndexAny}, nil
	case "last_index_any":
		return &Function{Name: name, Fn: s.LastIndexAny}, nil
	case "repeat":
		return &Function{Name: name, Fn: s.Repeat}, nil
	case "replace":
		return &Function{Name: name, Fn: s.Replace}, nil
	case "split":
		return &Function{Name: name, Fn: s.Split}, nil
	case "split_after":
		return &Function{Name: name, Fn: s.SplitAfter}, nil
	case "fields":
		return &Function{Name: name, Fn: s.Fields}, nil
	case "title":
		return &Function{Name: name, Fn: s.Title}, nil
	case "to_title":
		return &Function{Name: name, Fn: s.ToTitle}, nil
	case "to_valid_utf8":
		return &Function{Name: name, Fn: s.ToValidUTF8}, nil
	case "trim_left":
		return &Function{Name: name, Fn: s.TrimLeft}, nil
	case "trim_right":
		return &Function{Name: name, Fn: s.TrimRight}, nil
	case "trim_prefix":
		return &Function{Name: name, Fn: s.TrimPrefix}, nil
	case "trim_suffix":
		return &Function{Name: name, Fn: s.TrimSuffix}, nil
	case "cut":
		return &Function{Name: name, Fn: s.Cut}, nil
	case "cut_prefix":
		return &Function{Name: name, Fn: s.CutPrefix}, nil
	case "cut_suffix":
		return &Function{Name: name, Fn: s.CutSuffix}, nil
	case "constructor":
		return StrConstructorFn, nil
	default:
		return nil, NewAttributeError("String has no attribute '%s'", name)
	}
}

func (s String) Attributes() []string {
	return []string{
		"attributes", "size", "upper", "lower", "has_prefix", "has_suffix", "trim",
		"contains", "contains_any", "count", "equal_fold", "compare", "index", "last_index",
		"index_any", "last_index_any", "repeat", "replace", "split", "split_after", "fields",
		"title", "to_title", "to_valid_utf8", "trim_left", "trim_right", "trim_prefix",
		"trim_suffix", "cut", "cut_prefix", "cut_suffix", "constructor",
	}
}

var StrConstructorFn = &Function{Name: "Str", Fn: StrConstructor}

func StrConstructor(args CallArgs) (Object, error) {
	ap := NewArgParser("Str", args)
	value := ap.AnyOr("value", String(""))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	s, err := Repr(value)
	if err != nil {
		return nil, err
	}
	return String(s), nil
}
