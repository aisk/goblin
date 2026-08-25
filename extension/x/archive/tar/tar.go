package tar

import (
	"archive/tar"
	"bytes"
	"io"
	"strings"

	"github.com/aisk/goblin/extension/internal/archive"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "tar", Members: map[string]object.Object{
		"read_all":  &object.Function{Name: "read_all", Fn: tarReadAll},
		"write_all": &object.Function{Name: "write_all", Fn: tarWriteAll},
	}}, nil
}

func tarWriteAll(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("write_all", args)
	filesObj := p.Any("files")
	destObj := p.AnyOr("dest", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
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
	writer := tar.NewWriter(sink)
	for _, entry := range files {
		data := entry.Data
		header := &tar.Header{Name: entry.Name, Mode: 0o644, Size: int64(len(data))}
		if strings.HasSuffix(entry.Name, "/") {
			header.Typeflag = tar.TypeDir
			header.Mode = 0o755
			header.Size = 0
			data = nil
		}
		if err := writer.WriteHeader(header); err != nil {
			return nil, object.WrapNativeError(object.IOError, "write_all() failed to write header", err)
		}
		if _, err := writer.Write(data); err != nil {
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

func tarReadAll(args object.CallArgs) (object.Object, error) {
	data, err := archive.Data("read_all", args)
	if err != nil {
		return nil, err
	}
	reader := tar.NewReader(bytes.NewReader(data))
	var files []archive.Entry
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, object.WrapError(object.ParseError, "read_all() invalid tar data", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, object.WrapError(object.ParseError, "read_all() invalid tar data", err)
		}
		files = append(files, archive.Entry{Name: header.Name, Data: content})
	}
	return archive.Dict(files)
}
