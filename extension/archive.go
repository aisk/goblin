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
		"STORE":     object.Integer(zip.Store),
		"DEFLATE":   object.Integer(zip.Deflate),
	}}, nil
}

// archiveEntry preserves the caller's dict order so that produced archives are
// deterministic and read_all round-trips entries in archive order.
type archiveEntry struct {
	name string
	data []byte
}

func archiveData(name string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(name, args)
	data := p.BytesLike("data")
	return data, p.Finish()
}

func archiveFiles(name string, value object.Object) ([]archiveEntry, error) {
	dict, ok := value.(*object.Dict)
	if !ok {
		return nil, object.NewTypeError("%s() argument 'files' must be a dict, got %s", name, value.TypeName())
	}
	entries := make([]archiveEntry, 0, dict.Len())
	for _, entry := range dict.Entries() {
		filename, ok := entry.Key.(object.String)
		if !ok {
			return nil, object.NewTypeError("%s() file names must be strings, got %s", name, entry.Key.TypeName())
		}
		switch content := entry.Value.(type) {
		case object.Bytes:
			entries = append(entries, archiveEntry{string(filename), []byte(content)})
		case object.String:
			entries = append(entries, archiveEntry{string(filename), []byte(content)})
		default:
			return nil, object.NewTypeError("%s() file %q must contain Bytes or str, got %s", name, filename, entry.Value.TypeName())
		}
	}
	return entries, nil
}

func archiveDict(entries []archiveEntry) (*object.Dict, error) {
	result := object.NewDict()
	for _, entry := range entries {
		if err := result.Set(object.String(entry.name), object.NewBytes(entry.data)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// archiveDest resolves the optional dest keyword shared by the write_all
// functions: nil keeps buffering into output, anything else must be a duck
// writer that receives the archive bytes instead.
func archiveDest(destObj object.Object, output *bytes.Buffer) (io.Writer, error) {
	if _, ok := destObj.(object.Unit); ok {
		return output, nil
	}
	return object.NewDuckWriter("write_all", "dest", destObj)
}

func tarWriteAll(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("write_all", args)
	filesObj := p.Any("files")
	destObj := p.AnyOr("dest", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	files, err := archiveFiles("write_all", filesObj)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	sink, err := archiveDest(destObj, &output)
	if err != nil {
		return nil, err
	}
	writer := tar.NewWriter(sink)
	for _, entry := range files {
		data := entry.data
		header := &tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(data))}
		if strings.HasSuffix(entry.name, "/") {
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
	data, err := archiveData("read_all", args)
	if err != nil {
		return nil, err
	}
	reader := tar.NewReader(bytes.NewReader(data))
	var files []archiveEntry
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
		files = append(files, archiveEntry{header.Name, content})
	}
	return archiveDict(files)
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
	files, err := archiveFiles("write_all", filesObj)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	sink, err := archiveDest(destObj, &output)
	if err != nil {
		return nil, err
	}
	writer := zip.NewWriter(sink)
	for _, entry := range files {
		header := &zip.FileHeader{Name: entry.name, Method: uint16(method)}
		if strings.HasSuffix(entry.name, "/") {
			header.SetMode(0o755 | 1<<31)
		} else {
			header.SetMode(0o644)
		}
		stream, err := writer.CreateHeader(header)
		if err != nil {
			return nil, object.WrapNativeError(object.IOError, "write_all() failed to create entry", err)
		}
		if _, err := stream.Write(entry.data); err != nil {
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
	data, err := archiveData("read_all", args)
	if err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "read_all() invalid zip data", err)
	}
	var files []archiveEntry
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
		files = append(files, archiveEntry{file.Name, content})
	}
	return archiveDict(files)
}
