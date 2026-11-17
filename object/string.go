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
	trimStart
	trimEnd
)

// Size returns the length in runes, exposed to Goblin as the read-only
// property str.size.
func (s String) Size() Integer {
	return Integer(utf8.RuneCountInString(string(s)))
}

func (s String) Upper(args CallArgs) (Object, error) {
	if err := RequireNoArgs("upper", args); err != nil {
		return nil, err
	}
	return String(strings.ToUpper(string(s))), nil
}

func (s String) Lower(args CallArgs) (Object, error) {
	if err := RequireNoArgs("lower", args); err != nil {
		return nil, err
	}
	return String(strings.ToLower(string(s))), nil
}

// trim removes the characters in chars from one or both ends, Unicode
// whitespace when chars is nil.
func (s String) trim(name string, args CallArgs, direction trimDirection) (Object, error) {
	ap := NewArgParser(name, args)
	chars, hasChars := ap.NullableStr("chars")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	value := string(s)
	if !hasChars {
		switch direction {
		case trimStart:
			return String(strings.TrimLeftFunc(value, unicode.IsSpace)), nil
		case trimEnd:
			return String(strings.TrimRightFunc(value, unicode.IsSpace)), nil
		}
		return String(strings.TrimSpace(value)), nil
	}
	switch direction {
	case trimStart:
		return String(strings.TrimLeft(value, string(chars))), nil
	case trimEnd:
		return String(strings.TrimRight(value, string(chars))), nil
	}
	return String(strings.Trim(value, string(chars))), nil
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

func (s String) Contains(args CallArgs) (Object, error) {
	ap := NewArgParser("contains", args)
	value := ap.Str("value")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return stringSeq(s).contains(string(value)), nil
}

func (s String) Count(args CallArgs) (Object, error) {
	ap := NewArgParser("count", args)
	value := ap.Str("value")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return stringSeq(s).count(string(value)), nil
}

// IndexOf returns the character position of the first occurrence of value at
// or after start, or nil.
func (s String) IndexOf(args CallArgs) (Object, error) {
	ap := NewArgParser("index", args)
	value := ap.Str("value")
	start := ap.IntOr("start", 0)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return stringSeq(s).index(string(value), start)
}

func (s String) Replace(args CallArgs) (Object, error) {
	ap := NewArgParser("replace", args)
	old, newValue := ap.Str("old"), ap.Str("new")
	count, limited := ap.NullableInt("count")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return stringSeq(s).replace(string(old), string(newValue), count, limited)
}

func (s String) Split(args CallArgs) (Object, error) {
	ap := NewArgParser("split", args)
	sep, hasSep := ap.NullableStr("sep")
	count, limited := ap.NullableInt("count")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return stringSeq(s).split(string(sep), hasSep, count, limited)
}

func (s String) Cut(args CallArgs) (Object, error) {
	ap := NewArgParser("cut", args)
	sep := ap.Str("sep")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return stringSeq(s).cut(string(sep)), nil
}

func (s String) TypeName() string { return "String" }

func (s String) String() string {
	return string(s)
}

func (s String) ToString() (string, error) { return s.String(), nil }

// Literal returns the quoted Goblin source representation of the string.
func (s String) Literal() string {
	return strconv.Quote(string(s))
}

func (s String) ToBool() (bool, error) { return s != "", nil }

func (s String) Equals(other Object) (bool, error) {
	v, ok := other.(String)
	return ok && s == v, nil
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
		return 0, NewOperandError(OpCompare, "String", other)
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
		return nil, NewOperandError(OpAdd, "String", other)
	}
}

func (s String) Minus(other Object) (Object, error) {
	return nil, Unsupported("String", ErrFmtCannotSubtract)
}

func (s String) Multiply(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		if v < 0 {
			return nil, NewValueError("cannot multiply String by negative number")
		}
		if err := checkRepeat(len(s), v); err != nil {
			return nil, err
		}
		return String(strings.Repeat(string(s), int(v))), nil
	default:
		return nil, NewOperandError(OpMultiply, "String", other)
	}
}

// RMultiply repeats the string when it is the right operand, so `3 * "ab"`
// reads the same as `"ab" * 3`.
func (s String) RMultiply(left Object) (Object, bool, error) {
	if _, ok := left.(Integer); !ok {
		return nil, false, nil
	}
	result, err := s.Multiply(left)
	return result, true, err
}

func (s String) Divide(other Object) (Object, error) {
	return nil, Unsupported("String", ErrFmtCannotDivide)
}

func (s String) Modulo(other Object) (Object, error) {
	return nil, Unsupported("String", ErrFmtCannotModulo)
}

// A named string type cannot embed NoAssignment and NoReflectedOps. Only
// repetition reads in both operand orders (see RMultiply above); everything
// else a string does not accept is declined here.
func (s String) SetIndex(Object, Object) (bool, error) { return false, nil }
func (s String) SetAttr(string, Object) (bool, error)  { return false, nil }
func (s String) RAdd(Object) (Object, bool, error)     { return nil, false, nil }
func (s String) RMinus(Object) (Object, bool, error)   { return nil, false, nil }
func (s String) RDivide(Object) (Object, bool, error)  { return nil, false, nil }
func (s String) RModulo(Object) (Object, bool, error)  { return nil, false, nil }

// Iter returns the characters of s, as one-character Strings.
func (s String) Iter() ([]Object, error) { return Collect(s) }

// Iterator walks s character by character. A string is immutable, so the
// pass can read it lazily without changing what it visits.
func (s String) Iterator() (Iterator, error) {
	rest := string(s)
	return IteratorFunc(func() (Object, bool, error) {
		if rest == "" {
			return nil, false, nil
		}
		// Like a Go range over a string, an invalid byte reads as U+FFFD.
		r, size := utf8.DecodeRuneInString(rest)
		rest = rest[size:]
		return String(string(r)), true, nil
	}), nil
}

// Index returns the character at a position, as a one-character String.
// Positions count characters, as size and iteration do.
func (s String) Index(index Object) (Object, error) {
	idx, err := indexArg("String", index)
	if err != nil {
		return nil, err
	}
	str := string(s)
	i, err := ResolveIndex("String", idx, utf8.RuneCountInString(str))
	if err != nil {
		return nil, err
	}
	// Like iteration, an invalid byte reads as U+FFFD.
	r, _ := utf8.DecodeRuneInString(str[runeOffset(str, i):])
	return String(string(r)), nil
}

func (s String) GetAttr(name string) (Object, error) {
	switch name {
	case "size":
		return s.Size(), nil
	}
	if fn, ok := stringMethods.bind(s, name); ok {
		return fn, nil
	}
	return nil, NewAttributeError("String has no attribute '%s'", name)
}

func (s String) Attributes() []string { return stringMethods.attributes("size") }

var stringMethods = newMethodTable(
	"upper", "lower", "trim", "trim_start", "trim_end", "trim_prefix",
	"trim_suffix", "has_prefix", "has_suffix", "contains", "count", "index",
	"replace", "split", "cut",
)

var StringConstructorFn = &Function{Name: "String", Fn: StringConstructor}

func StringConstructor(args CallArgs) (Object, error) {
	ap := NewArgParser("String", args)
	value := ap.AnyOr("value", String(""))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	s, err := value.ToString()
	if err != nil {
		return nil, err
	}
	return String(s), nil
}

// Interpolate builds the value of a string literal with `{expression}` parts
// from the values of its parts, left to right: a String is taken as it is,
// any other value as the text print shows for it (ToString, through Show for
// a user type). Both backends evaluate the parts and call it.
func Interpolate(parts ...Object) (Object, error) {
	var b strings.Builder
	for _, part := range parts {
		if s, ok := part.(String); ok {
			b.WriteString(string(s))
			continue
		}
		s, err := part.ToString()
		if err != nil {
			return nil, err
		}
		b.WriteString(s)
	}
	return String(b.String()), nil
}
