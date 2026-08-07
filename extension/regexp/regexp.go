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
		"escape":  &object.Function{Name: "escape", Fn: escape},
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
		return nil, object.WrapError(object.ParseError, "compile() invalid pattern", err)
	}
	return &Pattern{
		OpaqueBase: object.MakeOpaqueBase("Pattern"),
		source:     string(source),
		re:         re,
		names:      re.SubexpNames(),
	}, nil
}

// escape quotes every metacharacter in text, so that compiling the result
// matches text literally. It is the safe way to build a pattern around input
// that is not itself a regular expression.
func escape(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("escape", args)
	text := p.Str("text")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.String(stdregexp.QuoteMeta(string(text))), nil
}
