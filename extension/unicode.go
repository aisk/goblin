package extension

import (
	"unicode"
	"unicode/utf8"

	"github.com/aisk/goblin/object"
)

func ExecuteUTF8() (object.Object, error) {
	return &object.Module{Name: "utf8", Members: map[string]object.Object{
		"valid":      &object.Function{Name: "valid", Fn: utf8Valid},
		"rune_count": &object.Function{Name: "rune_count", Fn: utf8RuneCount},
		"encode":     &object.Function{Name: "encode", Fn: utf8Encode},
		"decode":     &object.Function{Name: "decode", Fn: utf8Decode},
	}}, nil
}

func utf8Data(fnName string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(fnName, args)
	data := p.BytesLike("data")
	return data, p.Finish()
}

func utf8Valid(args object.CallArgs) (object.Object, error) {
	data, err := utf8Data("valid", args)
	if err != nil {
		return nil, err
	}
	return object.Bool(utf8.Valid(data)), nil
}

func utf8RuneCount(args object.CallArgs) (object.Object, error) {
	data, err := utf8Data("rune_count", args)
	if err != nil {
		return nil, err
	}
	return object.Integer(utf8.RuneCount(data)), nil
}

func utf8Encode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("encode", args)
	codepoint := p.Int("codepoint")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if codepoint < 0 || codepoint > utf8.MaxRune || codepoint >= 0xD800 && codepoint <= 0xDFFF {
		return nil, object.NewValueError("encode() invalid Unicode code point: %d", codepoint)
	}
	buffer := make([]byte, utf8.UTFMax)
	n := utf8.EncodeRune(buffer, rune(codepoint))
	return object.NewBytes(buffer[:n]), nil
}

func utf8Decode(args object.CallArgs) (object.Object, error) {
	data, err := utf8Data("decode", args)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, object.NewValueError("decode() data must not be empty")
	}
	r, size := utf8.DecodeRune(data)
	if r == utf8.RuneError && size == 1 {
		return nil, object.NewParseError("decode() invalid UTF-8 data")
	}
	return &object.List{Elements: []object.Object{object.Integer(r), object.Integer(size)}}, nil
}

func ExecuteUnicode() (object.Object, error) {
	return &object.Module{Name: "unicode", Members: map[string]object.Object{
		"is_letter":  &object.Function{Name: "is_letter", Fn: unicodePredicate("is_letter", unicode.IsLetter)},
		"is_digit":   &object.Function{Name: "is_digit", Fn: unicodePredicate("is_digit", unicode.IsDigit)},
		"is_number":  &object.Function{Name: "is_number", Fn: unicodePredicate("is_number", unicode.IsNumber)},
		"is_space":   &object.Function{Name: "is_space", Fn: unicodePredicate("is_space", unicode.IsSpace)},
		"is_upper":   &object.Function{Name: "is_upper", Fn: unicodePredicate("is_upper", unicode.IsUpper)},
		"is_lower":   &object.Function{Name: "is_lower", Fn: unicodePredicate("is_lower", unicode.IsLower)},
		"is_control": &object.Function{Name: "is_control", Fn: unicodePredicate("is_control", unicode.IsControl)},
		"to_upper":   &object.Function{Name: "to_upper", Fn: unicodeMapping("to_upper", unicode.ToUpper)},
		"to_lower":   &object.Function{Name: "to_lower", Fn: unicodeMapping("to_lower", unicode.ToLower)},
		"to_title":   &object.Function{Name: "to_title", Fn: unicodeMapping("to_title", unicode.ToTitle)},
	}}, nil
}

func unicodeRune(fnName string, args object.CallArgs) (rune, error) {
	p := object.NewArgParser(fnName, args)
	value := p.Str("character")
	if err := p.Finish(); err != nil {
		return 0, err
	}
	r, size := utf8.DecodeRuneInString(string(value))
	if len(value) == 0 || size != len(value) || r == utf8.RuneError && size == 1 {
		return 0, object.NewValueError("%s() argument 'character' must contain exactly one Unicode character", fnName)
	}
	return r, nil
}

func unicodePredicate(fnName string, predicate func(rune) bool) func(object.CallArgs) (object.Object, error) {
	return func(args object.CallArgs) (object.Object, error) {
		r, err := unicodeRune(fnName, args)
		if err != nil {
			return nil, err
		}
		return object.Bool(predicate(r)), nil
	}
}

func unicodeMapping(fnName string, mapping func(rune) rune) func(object.CallArgs) (object.Object, error) {
	return func(args object.CallArgs) (object.Object, error) {
		r, err := unicodeRune(fnName, args)
		if err != nil {
			return nil, err
		}
		return object.String(string(mapping(r))), nil
	}
}
