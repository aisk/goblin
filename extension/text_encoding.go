package extension

import (
	"bytes"
	"encoding/ascii85"
	"html"
	"io"
	"mime/quotedprintable"

	"github.com/aisk/goblin/object"
)

func ExecuteASCII85() (object.Object, error) {
	return &object.Module{Name: "ascii85", Members: map[string]object.Object{
		"encode": &object.Function{Name: "encode", Fn: ascii85Encode},
		"decode": &object.Function{Name: "decode", Fn: ascii85Decode},
	}}, nil
}

func ascii85Encode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("encode", args)
	data := p.BytesLike("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	dst := make([]byte, ascii85.MaxEncodedLen(len(data)))
	n := ascii85.Encode(dst, data)
	return object.String(dst[:n]), nil
}

func ascii85Decode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("decode", args)
	data := p.Str("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	dst := make([]byte, len(data))
	n, consumed, err := ascii85.Decode(dst, []byte(data), true)
	if err != nil {
		return nil, object.WrapError(object.ParseError, "decode() invalid ascii85 data", err)
	}
	if consumed != len(data) {
		return nil, object.NewParseError("decode() invalid ascii85 data")
	}
	return object.NewBytes(dst[:n]), nil
}

func ExecuteHTML() (object.Object, error) {
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

func ExecuteQuotedPrintable() (object.Object, error) {
	return &object.Module{Name: "quotedprintable", Members: map[string]object.Object{
		"encode": &object.Function{Name: "encode", Fn: quotedPrintableEncode},
		"decode": &object.Function{Name: "decode", Fn: quotedPrintableDecode},
	}}, nil
}

func quotedPrintableEncode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("encode", args)
	data := p.BytesLike("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	writer := quotedprintable.NewWriter(&output)
	if _, err := writer.Write(data); err != nil {
		return nil, object.WrapNativeError(object.IOError, "encode() failed to write data", err)
	}
	if err := writer.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "encode() failed to close stream", err)
	}
	return object.String(output.String()), nil
}

func quotedPrintableDecode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("decode", args)
	data := p.BytesLike("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	decoded, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(data)))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "decode() invalid quoted-printable data", err)
	}
	return object.NewBytes(decoded), nil
}
