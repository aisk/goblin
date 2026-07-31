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
	}}, nil
}

func lzwOptions(fnName string, args object.CallArgs) ([]byte, lzw.Order, int, error) {
	p := object.NewArgParser(fnName, args)
	dataObj := p.Any("data")
	orderName := p.StrOr("order", object.String("lsb"))
	litWidth := p.IntOr("lit_width", 8)
	if err := p.Finish(); err != nil {
		return nil, 0, 0, err
	}
	data, err := bytesOrString(fnName, "data", dataObj)
	if err != nil {
		return nil, 0, 0, err
	}
	var order lzw.Order
	switch orderName {
	case "lsb":
		order = lzw.LSB
	case "msb":
		order = lzw.MSB
	default:
		return nil, 0, 0, object.NewValueError("%s() argument 'order' must be \"lsb\" or \"msb\"", fnName)
	}
	if litWidth < 2 || litWidth > 8 {
		return nil, 0, 0, object.NewValueError("%s() argument 'lit_width' must be between 2 and 8", fnName)
	}
	return data, order, int(litWidth), nil
}

func lzwCompress(args object.CallArgs) (object.Object, error) {
	data, order, litWidth, err := lzwOptions("compress", args)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	writer := lzw.NewWriter(&output, order, litWidth)
	if _, err := writer.Write(data); err != nil {
		return nil, object.WrapNativeError(object.IOError, "compress() failed to write data", err)
	}
	if err := writer.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "compress() failed to close stream", err)
	}
	return object.NewBytes(output.Bytes()), nil
}

func lzwDecompress(args object.CallArgs) (object.Object, error) {
	data, order, litWidth, err := lzwOptions("decompress", args)
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
