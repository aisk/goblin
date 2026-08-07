package fs

import (
	"fmt"
	"io"
	"os"

	"github.com/aisk/goblin/object"
)

type File struct {
	object.OpaqueBase
	Name   string
	File   *os.File
	closed bool
}

func NewFile(name string, file *os.File) *File {
	return &File{OpaqueBase: object.MakeOpaqueBase("File"), Name: name, File: file}
}

func (f *File) ensureOpen(method string) error {
	if f.closed {
		return object.NewValueError("%s() called on closed file", method)
	}
	return nil
}

func (f *File) Read(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("read", args).Finish(); err != nil {
		return nil, err
	}
	if err := f.ensureOpen("read"); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(f.File)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "read() failed", err)
	}
	return object.String(data), nil
}

func (f *File) Write(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("write", args)
	content := ap.BytesLike("content")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if err := f.ensureOpen("write"); err != nil {
		return nil, err
	}

	n, err := f.File.Write(content)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "write() failed", err)
	}
	return object.Integer(n), nil
}

func (f *File) Close(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("close", args).Finish(); err != nil {
		return nil, err
	}
	if f.closed {
		return object.Nil, nil
	}
	if err := f.File.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "close() failed", err)
	}
	f.closed = true
	return object.Nil, nil
}

func (f *File) Stat(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("stat", args).Finish(); err != nil {
		return nil, err
	}
	if err := f.ensureOpen("stat"); err != nil {
		return nil, err
	}

	info, err := f.File.Stat()
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "stat() failed", err)
	}
	return NewFileInfo(info), nil
}

func (f *File) String() string {
	return fmt.Sprintf("<file %s>", f.Name)
}

func (f *File) ToString() (string, error) { return f.String(), nil }

func (f *File) ToBool() (bool, error) { return !f.closed, nil }

func (f *File) Equals(other object.Object) (bool, error) {
	v, ok := other.(*File)
	return ok && f == v, nil
}

func (f *File) Not() (object.Object, error) { return object.Bool(f.closed), nil }

func (f *File) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(f), nil
	case "name":
		return object.String(f.Name), nil
	case "closed":
		return object.Bool(f.closed), nil
	case "read":
		return &object.Function{Name: "read", Fn: f.Read}, nil
	case "write":
		return &object.Function{Name: "write", Fn: f.Write}, nil
	case "stat":
		return &object.Function{Name: "stat", Fn: f.Stat}, nil
	case "close":
		return &object.Function{Name: "close", Fn: f.Close}, nil
	default:
		return nil, object.NewAttributeError("File has no attribute '%s'", name)
	}
}

func (f *File) Attributes() []string {
	return []string{"attributes", "name", "closed", "read", "write", "stat", "close"}
}

var _ object.Object = (*File)(nil)
