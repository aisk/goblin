package html

import (
	"html"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "html", Members: map[string]object.Object{
		"escape":   &object.Function{Name: "escape", Fn: htmlEscape},
		"unescape": &object.Function{Name: "unescape", Fn: htmlUnescape},
	}}, nil
}

func htmlEscape(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("escape", args)
	value := p.Str("s")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.String(html.EscapeString(string(value))), nil
}

func htmlUnescape(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("unescape", args)
	value := p.Str("s")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.String(html.UnescapeString(string(value))), nil
}
