// Package url adapts Go's net/url package to Goblin.
package url

import (
	"net/url"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return object.NewModule("url", nil, object.Funcs{
		"parse":          parse,
		"join_path":      joinPath,
		"query_escape":   queryEscape,
		"query_unescape": queryUnescape,
		"path_escape":    pathEscape,
		"path_unescape":  pathUnescape,
	}), nil
}

func parse(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("parse", args)
	raw := p.Str("raw_url")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	value, err := url.Parse(string(raw))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "parse() invalid URL", err)
	}
	return newURL(value), nil
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
		return nil, object.NewTypeError("join_path() argument 'elements' must be List, got %s", elementsObj.TypeName())
	}
	elements := make([]string, len(list.Elements))
	for i, item := range list.Elements {
		value, ok := item.(object.String)
		if !ok {
			return nil, object.NewTypeError("join_path() argument 'elements' must be String, got %s at index %d", item.TypeName(), i)
		}
		elements[i] = string(value)
	}
	value, err := url.JoinPath(string(base), elements...)
	if err != nil {
		return nil, object.WrapError(object.ParseError, "join_path() invalid URL", err)
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
		return nil, object.WrapError(object.ParseError, name+"() invalid URL", err)
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
