package extension

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"
	"strings"

	"github.com/aisk/goblin/object"
)

func ExecuteTar() (object.Object, error) {
	return &object.Module{Name: "tar", Members: map[string]object.Object{
		"read_all":  &object.Function{Name: "read_all", Fn: tarReadAll},
		"write_all": &object.Function{Name: "write_all", Fn: tarWriteAll},
	}}, nil
}

func ExecuteZip() (object.Object, error) {
	return &object.Module{Name: "zip", Members: map[string]object.Object{
		"read_all":  &object.Function{Name: "read_all", Fn: zipReadAll},
		"write_all": &object.Function{Name: "write_all", Fn: zipWriteAll},
		"store":     object.Integer(zip.Store),
		"deflate":   object.Integer(zip.Deflate),
	}}, nil
}

func archiveData(name string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(name, args)
	value := p.Any("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case object.Bytes:
		return []byte(v), nil
	case object.String:
		return []byte(v), nil
	default:
		return nil, object.NewTypeError("%s() argument 'data' must be Bytes or str, got %T", name, value)
	}
}

func archiveFiles(name string, value object.Object) (map[string][]byte, error) {
	dict, ok := value.(*object.Dict)
	if !ok {
		return nil, object.NewTypeError("%s() argument 'files' must be a dict, got %T", name, value)
	}
	files := make(map[string][]byte, len(dict.Entries))
	for _, entry := range dict.Entries {
		filename, ok := entry.Key.(object.String)
		if !ok {
			return nil, object.NewTypeError("%s() file names must be strings, got %T", name, entry.Key)
		}
		switch content := entry.Value.(type) {
		case object.Bytes:
			files[string(filename)] = []byte(content)
		case object.String:
			files[string(filename)] = []byte(content)
		default:
			return nil, object.NewTypeError("%s() file %q must contain Bytes or str, got %T", name, filename, entry.Value)
		}
	}
	return files, nil
}

func archiveDict(files map[string][]byte) (*object.Dict, error) {
	result := object.NewDict()
	for name, data := range files {
		if err := result.Set(object.String(name), object.NewBytes(data)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func tarWriteAll(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("write_all", args)
	filesObj := p.Any("files")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	files, err := archiveFiles("write_all", filesObj)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	writer := tar.NewWriter(&output)
	for name, data := range files {
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}
		if strings.HasSuffix(name, "/") {
			header.Typeflag = tar.TypeDir
			header.Mode = 0o755
			header.Size = 0
			data = nil
		}
		if err := writer.WriteHeader(header); err != nil {
			return nil, object.WrapNativeError(object.IOError, "write_all() failed", err)
		}
		if _, err := writer.Write(data); err != nil {
			return nil, object.WrapNativeError(object.IOError, "write_all() failed", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "write_all() failed", err)
	}
	return object.NewBytes(output.Bytes()), nil
}

func tarReadAll(args object.CallArgs) (object.Object, error) {
	data, err := archiveData("read_all", args)
	if err != nil {
		return nil, err
	}
	reader := tar.NewReader(bytes.NewReader(data))
	files := make(map[string][]byte)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, object.WrapError(object.ParseError, "read_all() failed", err)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, object.WrapError(object.ParseError, "read_all() failed", err)
		}
		files[header.Name] = content
	}
	return archiveDict(files)
}

func zipWriteAll(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("write_all", args)
	filesObj := p.Any("files")
	method := p.IntOr("method", object.Integer(zip.Deflate))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if method != object.Integer(zip.Store) && method != object.Integer(zip.Deflate) {
		return nil, object.NewValueError("write_all() argument 'method' must be zip.store or zip.deflate")
	}
	files, err := archiveFiles("write_all", filesObj)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, data := range files {
		header := &zip.FileHeader{Name: name, Method: uint16(method)}
		if strings.HasSuffix(name, "/") {
			header.SetMode(0o755 | 1<<31)
		} else {
			header.SetMode(0o644)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return nil, object.WrapNativeError(object.IOError, "write_all() failed", err)
		}
		if _, err := entry.Write(data); err != nil {
			return nil, object.WrapNativeError(object.IOError, "write_all() failed", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "write_all() failed", err)
	}
	return object.NewBytes(output.Bytes()), nil
}

func zipReadAll(args object.CallArgs) (object.Object, error) {
	data, err := archiveData("read_all", args)
	if err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "read_all() failed", err)
	}
	files := make(map[string][]byte)
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return nil, object.WrapError(object.ParseError, "read_all() failed", err)
		}
		content, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil {
			return nil, object.WrapError(object.ParseError, "read_all() failed", readErr)
		}
		if closeErr != nil {
			return nil, object.WrapError(object.ParseError, "read_all() failed", closeErr)
		}
		files[file.Name] = content
	}
	return archiveDict(files)
}
