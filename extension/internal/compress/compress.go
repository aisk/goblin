// Package compress holds the shared plumbing of the whole-value compression
// modules under x/compress: argument parsing, the common module member set,
// and the buffer-or-dest write path.
package compress

import (
	"bytes"
	"compress/flate"
	"io"

	"github.com/aisk/goblin/object"
)

// Members builds the member set shared by the flate-based modules: the two
// functions plus the compression level constants.
func Members(compressFn, decompressFn func(object.CallArgs) (object.Object, error)) map[string]object.Object {
	return map[string]object.Object{
		"compress":            &object.Function{Name: "compress", Fn: compressFn},
		"decompress":          &object.Function{Name: "decompress", Fn: decompressFn},
		"NO_COMPRESSION":      object.Integer(flate.NoCompression),
		"BEST_SPEED":          object.Integer(flate.BestSpeed),
		"BEST_COMPRESSION":    object.Integer(flate.BestCompression),
		"DEFAULT_COMPRESSION": object.Integer(flate.DefaultCompression),
		"HUFFMAN_ONLY":        object.Integer(flate.HuffmanOnly),
	}
}

// Input parses the shared argument shape: data, plus level and dest when
// compressing.
func Input(name string, args object.CallArgs, compressing bool) ([]byte, int, *object.DuckWriter, error) {
	p := object.NewArgParser(name, args)
	data := p.BytesLike("data")
	level := object.Integer(flate.DefaultCompression)
	destObj := object.Object(object.Nil)
	if compressing {
		level = p.IntOr("level", flate.DefaultCompression)
		destObj = p.AnyOr("dest", object.Nil)
	}
	if err := p.Finish(); err != nil {
		return nil, 0, nil, err
	}
	if compressing && (int(level) < flate.HuffmanOnly || int(level) > flate.BestCompression) {
		return nil, 0, nil, object.NewValueError("%s() argument 'level' must be between %d and %d, got %d", name, flate.HuffmanOnly, flate.BestCompression, int(level))
	}
	var dest *object.DuckWriter
	if _, ok := destObj.(object.Unit); !ok {
		var err error
		dest, err = object.NewDuckWriter(name, "dest", destObj)
		if err != nil {
			return nil, 0, nil, err
		}
	}
	return data, int(level), dest, nil
}

// Compress runs the shared compress path over a format-specific writer.
func Compress(name string, args object.CallArgs, newWriter func(io.Writer, int) (io.WriteCloser, error)) (object.Object, error) {
	data, level, dest, err := Input(name, args, true)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	var sink io.Writer = &output
	if dest != nil {
		sink = dest
	}
	writer, err := newWriter(sink, level)
	if err != nil {
		return nil, object.WrapError(object.ValueError, name+"() invalid compression level", err)
	}
	if _, err := writer.Write(data); err != nil {
		return nil, object.WrapNativeError(object.IOError, name+"() failed to write data", err)
	}
	if err := writer.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, name+"() failed to close stream", err)
	}
	if dest != nil {
		return object.Nil, nil
	}
	return object.NewBytes(output.Bytes()), nil
}

// Decompress runs the shared decompress path over a format-specific reader.
func Decompress(name string, args object.CallArgs, newReader func(io.Reader) (io.ReadCloser, error)) (object.Object, error) {
	data, _, _, err := Input(name, args, false)
	if err != nil {
		return nil, err
	}
	reader, err := newReader(bytes.NewReader(data))
	if err != nil {
		return nil, object.WrapError(object.ParseError, name+"() invalid compressed data", err)
	}
	defer reader.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		return nil, object.WrapError(object.ParseError, name+"() invalid compressed data", err)
	}
	return object.NewBytes(output), nil
}
