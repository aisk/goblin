package gzip

import (
	"compress/gzip"
	"io"

	"github.com/aisk/goblin/extension/internal/compress"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "gzip", Members: compress.Members(gzipCompress, gzipDecompress)}, nil
}

func gzipCompress(args object.CallArgs) (object.Object, error) {
	return compress.Compress("compress", args, func(w io.Writer, level int) (io.WriteCloser, error) {
		return gzip.NewWriterLevel(w, level)
	})
}

func gzipDecompress(args object.CallArgs) (object.Object, error) {
	return compress.Decompress("decompress", args, func(r io.Reader) (io.ReadCloser, error) {
		return gzip.NewReader(r)
	})
}
