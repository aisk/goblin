package zlib

import (
	"compress/zlib"
	"io"

	"github.com/aisk/goblin/extension/internal/compress"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "zlib", Members: compress.Members(zlibCompress, zlibDecompress)}, nil
}

func zlibCompress(args object.CallArgs) (object.Object, error) {
	return compress.Compress("compress", args, func(w io.Writer, level int) (io.WriteCloser, error) {
		return zlib.NewWriterLevel(w, level)
	})
}

func zlibDecompress(args object.CallArgs) (object.Object, error) {
	return compress.Decompress("decompress", args, func(r io.Reader) (io.ReadCloser, error) {
		return zlib.NewReader(r)
	})
}
