package fs

import (
	"fmt"
	"io"
	"os"

	"github.com/aisk/goblin/object"
)

type File struct {
	Name   string
	File   *os.File
	closed bool
}

func NewFile(name string, file *os.File) *File {
	return &File{Name: name, File: file}
}

func (f *File) ensureOpen(method string) error {
	if f.closed {
		return object.NewValueError("%s() called on closed file", method)
	}
	return nil
}

func (f *File) Read(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("read", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("read() takes exactly 0 arguments, got %d", len(args.Positional))
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
	contentArg := ap.Any("content")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if err := f.ensureOpen("write"); err != nil {
		return nil, err
	}

	content, ok := contentArg.(object.String)
	if !ok {
		return nil, object.NewTypeError("write() argument 'content' must be a string, got %T", contentArg)
	}

	n, err := f.File.WriteString(string(content))
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "write() failed", err)
	}
	return object.Integer(n), nil
}

func (f *File) Close(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("close", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("close() takes exactly 0 arguments, got %d", len(args.Positional))
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
	if err := object.RequireNoKeyword("stat", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("stat() takes exactly 0 arguments, got %d", len(args.Positional))
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

func (f *File) Bool() bool {
	return !f.closed
}

func (f *File) ToBool() (bool, error) { return f.Bool(), nil }

func (f *File) Compare(object.Object) (int, error) {
	return 0, object.NewTypeError("cannot compare File")
}

func (f *File) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot add File")
}
func (f *File) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot subtract File")
}
func (f *File) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot multiply File")
}
func (f *File) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot divide File")
}
func (f *File) And(other object.Object) (object.Object, error) {
	return object.Bool(f.Bool() && other.Bool()), nil
}
func (f *File) Or(other object.Object) (object.Object, error) {
	return object.Bool(f.Bool() || other.Bool()), nil
}
func (f *File) Not() (object.Object, error) { return object.Bool(!f.Bool()), nil }
func (f *File) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("File does not support iteration")
}
func (f *File) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("File is not indexable")
}

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
