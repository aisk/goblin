// Package regexp exposes Go's RE2-based regular expression engine to Goblin.
package regexp

import (
	stdregexp "regexp"

	"github.com/aisk/goblin/object"
)

// Execute builds the regexp standard-library module.
func Execute() (object.Object, error) {
	return &object.Module{Name: "regexp", Members: map[string]object.Object{
		"compile": &object.Function{Name: "compile", Fn: compile},
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
	full, err := stdregexp.Compile(`\A(?:` + string(source) + `)\z`)
	if err != nil {
		return nil, object.WrapError(object.ParseError, "compile() failed", err)
	}
	return &Pattern{source: string(source), re: re, full: full}, nil
}
