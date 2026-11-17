// Package url is the url module: take a URL apart, escape its parts and
// change its query. A URL is a value; it never touches the network.
package url

import (
	"net/url"

	"github.com/aisk/goblin/extension/internal/urlvalue"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return object.NewModule("url", map[string]object.Object{
		"URL": urlvalue.Type.Function,
	}, object.Funcs{
		"parse":        parse,
		"encode_query": encodeQuery,
		"decode_query": decodeQuery,
		"query_values": queryValues,
		"query_escape": queryEscape,
		"path_escape":  pathEscape,
	}), nil
}

func textArg(fn string, args object.CallArgs) (string, error) {
	p := object.NewArgParser(fn, args)
	text := p.Str("text")
	if err := p.Finish(); err != nil {
		return "", err
	}
	return string(text), nil
}

func parse(args object.CallArgs) (object.Object, error) {
	text, err := textArg("parse", args)
	if err != nil {
		return nil, err
	}
	return urlvalue.Parse("parse", text)
}

func encodeQuery(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("encode_query", args)
	params := p.Dict("params")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	query, err := urlvalue.EncodeQuery("encode_query", "params", params)
	if err != nil {
		return nil, err
	}
	return object.String(query), nil
}

func decodeQuery(args object.CallArgs) (object.Object, error) {
	text, err := textArg("decode_query", args)
	if err != nil {
		return nil, err
	}
	return urlvalue.DecodeQuery("decode_query", text)
}

func queryEscape(args object.CallArgs) (object.Object, error) {
	text, err := textArg("query_escape", args)
	if err != nil {
		return nil, err
	}
	return object.String(url.QueryEscape(text)), nil
}

func pathEscape(args object.CallArgs) (object.Object, error) {
	text, err := textArg("path_escape", args)
	if err != nil {
		return nil, err
	}
	return object.String(url.PathEscape(text)), nil
}

// query_values(text, name) is every value name has in the query string text.
func queryValues(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("query_values", args)
	text := p.Str("text")
	name := p.Str("name")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return urlvalue.QueryValues("query_values", string(text), string(name))
}
