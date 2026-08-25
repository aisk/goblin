package bzip2

import (
	"bytes"
	"compress/bzip2"
	"io"

	"github.com/aisk/goblin/extension/internal/compress"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "bzip2", Members: map[string]object.Object{
		"decompress": &object.Function{Name: "decompress", Fn: bzip2Decompress},
	}}, nil
}

func bzip2Decompress(args object.CallArgs) (object.Object, error) {
	data, _, _, err := compress.Input("decompress", args, false)
	if err != nil {
		return nil, err
	}
	output, err := io.ReadAll(bzip2.NewReader(bytes.NewReader(data)))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "decompress() invalid bzip2 data", err)
	}
	return object.NewBytes(output), nil
}
