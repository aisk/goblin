package extension

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"

	"github.com/aisk/goblin/object"
)

func compressionMembers(compressFn, decompressFn func(object.CallArgs) (object.Object, error)) map[string]object.Object {
	return map[string]object.Object{
		"compress":            &object.Function{Name: "compress", Fn: compressFn},
		"decompress":          &object.Function{Name: "decompress", Fn: decompressFn},
		"no_compression":      object.Integer(flate.NoCompression),
		"best_speed":          object.Integer(flate.BestSpeed),
		"best_compression":    object.Integer(flate.BestCompression),
		"default_compression": object.Integer(flate.DefaultCompression),
		"huffman_only":        object.Integer(flate.HuffmanOnly),
	}
}

func ExecuteGzip() (object.Object, error) {
	return &object.Module{Name: "gzip", Members: compressionMembers(gzipCompress, gzipDecompress)}, nil
}

func ExecuteZlib() (object.Object, error) {
	return &object.Module{Name: "zlib", Members: compressionMembers(zlibCompress, zlibDecompress)}, nil
}

func compressionInput(name string, args object.CallArgs, withLevel bool) ([]byte, int, error) {
	p := object.NewArgParser(name, args)
	value := p.Any("data")
	level := object.Integer(flate.DefaultCompression)
	if withLevel {
		level = p.IntOr("level", flate.DefaultCompression)
	}
	if err := p.Finish(); err != nil {
		return nil, 0, err
	}
	var data []byte
	switch v := value.(type) {
	case object.Bytes:
		data = []byte(v)
	case object.String:
		data = []byte(v)
	default:
		return nil, 0, object.NewTypeError("%s() argument 'data' must be Bytes or str, got %T", name, value)
	}
	return data, int(level), nil
}

func compressedBytes(name string, args object.CallArgs, newWriter func(io.Writer, int) (io.WriteCloser, error)) (object.Object, error) {
	data, level, err := compressionInput(name, args, true)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	writer, err := newWriter(&output, level)
	if err != nil {
		return nil, object.WrapError(object.ValueError, name+"() failed", err)
	}
	if _, err := writer.Write(data); err != nil {
		return nil, object.WrapNativeError(object.IOError, name+"() failed", err)
	}
	if err := writer.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, name+"() failed", err)
	}
	return object.NewBytes(output.Bytes()), nil
}

func decompressedBytes(name string, args object.CallArgs, newReader func(io.Reader) (io.ReadCloser, error)) (object.Object, error) {
	data, _, err := compressionInput(name, args, false)
	if err != nil {
		return nil, err
	}
	reader, err := newReader(bytes.NewReader(data))
	if err != nil {
		return nil, object.WrapError(object.ParseError, name+"() failed", err)
	}
	defer reader.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		return nil, object.WrapError(object.ParseError, name+"() failed", err)
	}
	return object.NewBytes(output), nil
}

func gzipCompress(args object.CallArgs) (object.Object, error) {
	return compressedBytes("compress", args, func(w io.Writer, level int) (io.WriteCloser, error) {
		return gzip.NewWriterLevel(w, level)
	})
}
func gzipDecompress(args object.CallArgs) (object.Object, error) {
	return decompressedBytes("decompress", args, func(r io.Reader) (io.ReadCloser, error) {
		return gzip.NewReader(r)
	})
}
func zlibCompress(args object.CallArgs) (object.Object, error) {
	return compressedBytes("compress", args, func(w io.Writer, level int) (io.WriteCloser, error) {
		return zlib.NewWriterLevel(w, level)
	})
}
func zlibDecompress(args object.CallArgs) (object.Object, error) {
	return decompressedBytes("decompress", args, func(r io.Reader) (io.ReadCloser, error) {
		return zlib.NewReader(r)
	})
}
