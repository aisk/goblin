// Package gzip implements the gzip module: whole values in gzip format.
package gzip

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return object.NewModule("gzip", nil, object.Funcs{
		"compress":   compress,
		"decompress": decompress,
	}), nil
}

func compress(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("compress", args)
	data := p.Data("data")
	level, hasLevel := p.NullableInt("level")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	goLevel := gzip.DefaultCompression
	if hasLevel {
		if level < gzip.BestSpeed || level > gzip.BestCompression {
			return nil, object.NewValueError("compress() argument 'level' must be from 1 to 9, got %d", int64(level))
		}
		goLevel = int(level)
	}
	var out bytes.Buffer
	w, err := gzip.NewWriterLevel(&out, goLevel)
	if err != nil {
		return nil, object.WrapError(object.ValueError, "compress() invalid level", err)
	}
	if _, err := io.Copy(w, data); err != nil {
		var goblinErr *object.Error
		if errors.As(err, &goblinErr) {
			return nil, err
		}
		return nil, object.WrapNativeError(object.IOError, "compress() cannot read data", err)
	}
	if err := w.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "compress() cannot write", err)
	}
	return object.NewBytes(out.Bytes()), nil
}

// decompress takes Bytes or a readable; gzip data is binary, so a String is
// rejected rather than read as its UTF-8 bytes.
func decompress(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("decompress", args)
	value := p.Any("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	var input io.Reader
	if _, isString := value.(object.String); !isString {
		input, _ = object.DataReader("decompress", "data", value)
	}
	if input == nil {
		return nil, object.NewTypeError("decompress() argument 'data' must be Bytes or have a read(size) method, got %s", value.TypeName())
	}
	r, err := gzip.NewReader(input)
	if err != nil {
		return nil, decompressError(err, true)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, decompressError(err, false)
	}
	return object.NewBytes(out), nil
}

// decompressError passes on an error the readable raised and reports any
// other failure as a ParseError: input that does not start like gzip is "not
// gzip data", anything wrong after that is "corrupt gzip data".
func decompressError(err error, atStart bool) error {
	var goblinErr *object.Error
	if errors.As(err, &goblinErr) {
		return err
	}
	if atStart && (errors.Is(err, gzip.ErrHeader) || err == io.EOF) {
		return object.NewParseError("decompress() not gzip data")
	}
	return object.NewParseError("decompress() corrupt gzip data")
}
