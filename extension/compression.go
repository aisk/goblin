package extension

import (
	"bytes"
	"compress/bzip2"
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
		"NO_COMPRESSION":      object.Integer(flate.NoCompression),
		"BEST_SPEED":          object.Integer(flate.BestSpeed),
		"BEST_COMPRESSION":    object.Integer(flate.BestCompression),
		"DEFAULT_COMPRESSION": object.Integer(flate.DefaultCompression),
		"HUFFMAN_ONLY":        object.Integer(flate.HuffmanOnly),
	}
}

func ExecuteGzip() (object.Object, error) {
	return &object.Module{Name: "gzip", Members: compressionMembers(gzipCompress, gzipDecompress)}, nil
}

func ExecuteZlib() (object.Object, error) {
	return &object.Module{Name: "zlib", Members: compressionMembers(zlibCompress, zlibDecompress)}, nil
}

func ExecuteFlate() (object.Object, error) {
	return &object.Module{Name: "flate", Members: compressionMembers(flateCompress, flateDecompress)}, nil
}

func ExecuteBzip2() (object.Object, error) {
	return &object.Module{Name: "bzip2", Members: map[string]object.Object{
		"decompress": &object.Function{Name: "decompress", Fn: bzip2Decompress},
	}}, nil
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
	if withLevel && (int(level) < flate.HuffmanOnly || int(level) > flate.BestCompression) {
		return nil, 0, object.NewValueError("%s() argument 'level' must be between %d and %d, got %d", name, flate.HuffmanOnly, flate.BestCompression, int(level))
	}
	var data []byte
	switch v := value.(type) {
	case object.Bytes:
		data = []byte(v)
	case object.String:
		data = []byte(v)
	default:
		return nil, 0, object.NewTypeError("%s() argument 'data' must be Bytes or str, got %s", name, value.TypeName())
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
		return nil, object.WrapError(object.ValueError, name+"() invalid compression level", err)
	}
	if _, err := writer.Write(data); err != nil {
		return nil, object.WrapNativeError(object.IOError, name+"() failed to write data", err)
	}
	if err := writer.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, name+"() failed to close stream", err)
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
		return nil, object.WrapError(object.ParseError, name+"() invalid compressed data", err)
	}
	defer reader.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		return nil, object.WrapError(object.ParseError, name+"() invalid compressed data", err)
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

func flateCompress(args object.CallArgs) (object.Object, error) {
	return compressedBytes("compress", args, func(w io.Writer, level int) (io.WriteCloser, error) {
		return flate.NewWriter(w, level)
	})
}

func flateDecompress(args object.CallArgs) (object.Object, error) {
	return decompressedBytes("decompress", args, func(r io.Reader) (io.ReadCloser, error) {
		return flate.NewReader(r), nil
	})
}

func bzip2Decompress(args object.CallArgs) (object.Object, error) {
	data, _, err := compressionInput("decompress", args, false)
	if err != nil {
		return nil, err
	}
	output, err := io.ReadAll(bzip2.NewReader(bytes.NewReader(data)))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "decompress() invalid bzip2 data", err)
	}
	return object.NewBytes(output), nil
}
