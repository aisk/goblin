package quotedprintable

import (
	"bytes"
	"io"
	"mime/quotedprintable"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
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
