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

func compressionInput(name string, args object.CallArgs, compressing bool) ([]byte, int, *object.DuckWriter, error) {
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

func compressedBytes(name string, args object.CallArgs, newWriter func(io.Writer, int) (io.WriteCloser, error)) (object.Object, error) {
	data, level, dest, err := compressionInput(name, args, true)
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

func decompressedBytes(name string, args object.CallArgs, newReader func(io.Reader) (io.ReadCloser, error)) (object.Object, error) {
	data, _, _, err := compressionInput(name, args, false)
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
	data, _, _, err := compressionInput("decompress", args, false)
	if err != nil {
		return nil, err
	}
	output, err := io.ReadAll(bzip2.NewReader(bytes.NewReader(data)))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "decompress() invalid bzip2 data", err)
	}
	return object.NewBytes(output), nil
}
