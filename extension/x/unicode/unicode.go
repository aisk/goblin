package unicode

import (
	"unicode"
	"unicode/utf8"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
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
