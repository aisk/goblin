// Package url adapts Go's net/url package to Goblin.
package url

import (
	"net/url"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "url", Members: map[string]object.Object{
		"parse":          &object.Function{Name: "parse", Fn: parse},
		"join_path":      &object.Function{Name: "join_path", Fn: joinPath},
		"query_escape":   &object.Function{Name: "query_escape", Fn: queryEscape},
		"query_unescape": &object.Function{Name: "query_unescape", Fn: queryUnescape},
		"path_escape":    &object.Function{Name: "path_escape", Fn: pathEscape},
		"path_unescape":  &object.Function{Name: "path_unescape", Fn: pathUnescape},
	}}, nil
}

func parse(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("parse", args)
	raw := p.Str("raw_url")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	value, err := url.Parse(string(raw))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "parse() failed", err)
	}
	return &URL{value: value}, nil
}

func joinPath(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("join_path", args)
	base := p.Str("base")
	elementsObj := p.Any("elements")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	list, ok := elementsObj.(*object.List)
	if !ok {
		return nil, object.NewTypeError("join_path() argument 'elements' must be a list, got %T", elementsObj)
	}
	elements := make([]string, len(list.Elements))
	for i, item := range list.Elements {
		value, ok := item.(object.String)
		if !ok {
			return nil, object.NewTypeError("join_path() argument 'elements' must contain strings, got %T at index %d", item, i)
		}
		elements[i] = string(value)
	}
	value, err := url.JoinPath(string(base), elements...)
	if err != nil {
		return nil, object.WrapError(object.ParseError, "join_path() failed", err)
	}
	return object.String(value), nil
}

func transform(name string, args object.CallArgs, fn func(string) string) (object.Object, error) {
	p := object.NewArgParser(name, args)
	value := p.Str("s")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.String(fn(string(value))), nil
}

func unescape(name string, args object.CallArgs, fn func(string) (string, error)) (object.Object, error) {
	p := object.NewArgParser(name, args)
	value := p.Str("s")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	result, err := fn(string(value))
	if err != nil {
		return nil, object.WrapError(object.ParseError, name+"() failed", err)
	}
	return object.String(result), nil
}

func queryEscape(args object.CallArgs) (object.Object, error) {
	return transform("query_escape", args, url.QueryEscape)
}
func pathEscape(args object.CallArgs) (object.Object, error) {
	return transform("path_escape", args, url.PathEscape)
}
func queryUnescape(args object.CallArgs) (object.Object, error) {
	return unescape("query_unescape", args, url.QueryUnescape)
}
func pathUnescape(args object.CallArgs) (object.Object, error) {
	return unescape("path_unescape", args, url.PathUnescape)
}
