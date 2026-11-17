// Package base64 implements the base64 module: bytes as Base64 text.
package base64

import (
	"encoding/base64"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return object.NewModule("base64", nil, object.Funcs{
		"encode": encode,
		"decode": decode,
	}), nil
}

func alphabet(urlSafe bool) *base64.Encoding {
	if urlSafe {
		return base64.URLEncoding
	}
	return base64.StdEncoding
}

func encode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("encode", args)
	data := p.DataBytes("data")
	urlSafe := p.BoolOr("url_safe", false)
	padding := p.BoolOr("padding", true)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	enc := alphabet(bool(urlSafe))
	if !padding {
		enc = enc.WithPadding(base64.NoPadding)
	}
	return object.String(enc.EncodeToString(data)), nil
}

// decode accepts text with or without its trailing "=" padding: text that
// ends in "=" must be padded correctly, text that does not is read unpadded.
// Line breaks anywhere are skipped by the Go decoder.
func decode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("decode", args)
	text := string(p.Str("text"))
	urlSafe := p.BoolOr("url_safe", false)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	enc := alphabet(bool(urlSafe))
	if !strings.HasSuffix(strings.TrimRight(text, "\r\n"), "=") {
		enc = enc.WithPadding(base64.NoPadding)
	}
	data, err := enc.DecodeString(text)
	if err != nil {
		var corrupt base64.CorruptInputError
		if errors.As(err, &corrupt) {
			// Report the position as a character index into text.
			offset := min(int(corrupt), len(text))
			return nil, object.NewParseError("decode() invalid Base64 at position %d", utf8.RuneCountInString(text[:offset]))
		}
		return nil, object.WrapError(object.ParseError, "decode() invalid Base64", err)
	}
	return object.NewBytes(data), nil
}
