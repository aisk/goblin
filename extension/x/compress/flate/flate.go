package flate

import (
	"compress/flate"
	"io"

	"github.com/aisk/goblin/extension/internal/compress"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "flate", Members: compress.Members(flateCompress, flateDecompress)}, nil
}

func flateCompress(args object.CallArgs) (object.Object, error) {
	return compress.Compress("compress", args, func(w io.Writer, level int) (io.WriteCloser, error) {
		return flate.NewWriter(w, level)
	})
}

func flateDecompress(args object.CallArgs) (object.Object, error) {
	return compress.Decompress("decompress", args, func(r io.Reader) (io.ReadCloser, error) {
		return flate.NewReader(r), nil
	})
}
