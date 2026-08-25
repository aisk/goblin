package zip

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"

	"github.com/aisk/goblin/extension/internal/archive"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "zip", Members: map[string]object.Object{
		"read_all":  &object.Function{Name: "read_all", Fn: zipReadAll},
		"write_all": &object.Function{Name: "write_all", Fn: zipWriteAll},
		"STORE":     object.Integer(zip.Store),
		"DEFLATE":   object.Integer(zip.Deflate),
	}}, nil
}

func zipWriteAll(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("write_all", args)
	filesObj := p.Any("files")
	method := p.IntOr("method", object.Integer(zip.Deflate))
	destObj := p.AnyOr("dest", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if method != object.Integer(zip.Store) && method != object.Integer(zip.Deflate) {
		return nil, object.NewValueError("write_all() argument 'method' must be zip.STORE or zip.DEFLATE")
	}
	files, err := archive.Files("write_all", filesObj)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	sink, err := archive.Dest(destObj, &output)
	if err != nil {
		return nil, err
	}
	writer := zip.NewWriter(sink)
	for _, entry := range files {
		header := &zip.FileHeader{Name: entry.Name, Method: uint16(method)}
		if strings.HasSuffix(entry.Name, "/") {
			header.SetMode(0o755 | 1<<31)
		} else {
			header.SetMode(0o644)
		}
		stream, err := writer.CreateHeader(header)
		if err != nil {
			return nil, object.WrapNativeError(object.IOError, "write_all() failed to create entry", err)
		}
		if _, err := stream.Write(entry.Data); err != nil {
			return nil, object.WrapNativeError(object.IOError, "write_all() failed to write data", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "write_all() failed to close archive", err)
	}
	if sink != &output {
		return object.Nil, nil
	}
	return object.NewBytes(output.Bytes()), nil
}

func zipReadAll(args object.CallArgs) (object.Object, error) {
	data, err := archive.Data("read_all", args)
	if err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "read_all() invalid zip data", err)
	}
	var files []archive.Entry
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return nil, object.WrapError(object.ParseError, "read_all() invalid zip data", err)
		}
		content, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil {
			return nil, object.WrapError(object.ParseError, "read_all() invalid zip data", readErr)
		}
		if closeErr != nil {
			return nil, object.WrapError(object.ParseError, "read_all() invalid zip data", closeErr)
		}
		files = append(files, archive.Entry{Name: file.Name, Data: content})
	}
	return archive.Dict(files)
}
