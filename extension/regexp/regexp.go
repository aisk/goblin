// Package regexp exposes Go's RE2-based regular expression engine to Goblin.
package regexp

import (
	stdregexp "regexp"

	"github.com/aisk/goblin/object"
)

// Execute builds the regexp standard-library module.
func Execute() (object.Object, error) {
	return &object.Module{Name: "regexp", Members: map[string]object.Object{
		"compile":      &object.Function{Name: "compile", Fn: compile},
		"match_string": &object.Function{Name: "match_string", Fn: matchString},
		"quote_meta":   &object.Function{Name: "quote_meta", Fn: quoteMeta},
	}}, nil
}

func compile(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("compile", args)
	source := p.Str("pattern")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	re, err := stdregexp.Compile(string(source))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "compile() failed", err)
	}
	return &Regexp{source: string(source), re: re}, nil
}

func matchString(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("match_string", args)
	pattern := p.Str("pattern")
	text := p.Str("text")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	matched, err := stdregexp.MatchString(string(pattern), string(text))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "match_string() failed", err)
	}
	return object.Bool(matched), nil
}

func quoteMeta(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("quote_meta", args)
	text := p.Str("text")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.String(stdregexp.QuoteMeta(string(text))), nil
}
