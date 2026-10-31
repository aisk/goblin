package html

import (
	"html"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return object.NewModule("html", nil, object.Funcs{
		"escape":   htmlEscape,
		"unescape": htmlUnescape,
	}), nil
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
