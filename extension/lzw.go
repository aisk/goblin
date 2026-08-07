package extension

import (
	"bytes"
	"compress/lzw"
	"io"

	"github.com/aisk/goblin/object"
)

func ExecuteLZW() (object.Object, error) {
	return &object.Module{Name: "lzw", Members: map[string]object.Object{
		"compress":   &object.Function{Name: "compress", Fn: lzwCompress},
		"decompress": &object.Function{Name: "decompress", Fn: lzwDecompress},
		"LSB":        object.String("lsb"),
		"MSB":        object.String("msb"),
	}}, nil
}

func lzwOptions(fnName string, args object.CallArgs, compressing bool) ([]byte, lzw.Order, int, *object.DuckWriter, error) {
	p := object.NewArgParser(fnName, args)
	data := p.BytesLike("data")
	orderName := p.StrOr("order", object.String("lsb"))
	litWidth := p.IntOr("lit_width", 8)
	destObj := object.Object(object.Nil)
	if compressing {
		destObj = p.AnyOr("dest", object.Nil)
	}
	if err := p.Finish(); err != nil {
		return nil, 0, 0, nil, err
	}
	var order lzw.Order
	switch orderName {
	case "lsb":
		order = lzw.LSB
	case "msb":
		order = lzw.MSB
	default:
		return nil, 0, 0, nil, object.NewValueError("%s() argument 'order' must be lzw.LSB or lzw.MSB", fnName)
	}
	if litWidth < 2 || litWidth > 8 {
		return nil, 0, 0, nil, object.NewValueError("%s() argument 'lit_width' must be between 2 and 8", fnName)
	}
	var dest *object.DuckWriter
	if _, ok := destObj.(object.Unit); !ok {
		var err error
		dest, err = object.NewDuckWriter(fnName, "dest", destObj)
		if err != nil {
			return nil, 0, 0, nil, err
		}
	}
	return data, order, int(litWidth), dest, nil
}

func lzwCompress(args object.CallArgs) (object.Object, error) {
	data, order, litWidth, dest, err := lzwOptions("compress", args, true)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	var sink io.Writer = &output
	if dest != nil {
		sink = dest
	}
	writer := lzw.NewWriter(sink, order, litWidth)
	if _, err := writer.Write(data); err != nil {
		return nil, object.WrapNativeError(object.IOError, "compress() failed to write data", err)
	}
	if err := writer.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "compress() failed to close stream", err)
	}
	if dest != nil {
		return object.Nil, nil
	}
	return object.NewBytes(output.Bytes()), nil
}

func lzwDecompress(args object.CallArgs) (object.Object, error) {
	data, order, litWidth, _, err := lzwOptions("decompress", args, false)
	if err != nil {
		return nil, err
	}
	reader := lzw.NewReader(bytes.NewReader(data), order, litWidth)
	value, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, object.WrapError(object.ParseError, "decompress() invalid lzw data", readErr)
	}
	if closeErr != nil {
		return nil, object.WrapError(object.ParseError, "decompress() invalid lzw data", closeErr)
	}
	return object.NewBytes(value), nil
}
