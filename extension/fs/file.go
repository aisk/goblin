package fs

import (
	"fmt"
	"io"
	stdfs "io/fs"

	"github.com/aisk/goblin/object"
)

type File struct {
	Name   string
	File   stdfs.File
	closed bool
}

func NewFile(name string, file stdfs.File) *File {
	return &File{Name: name, File: file}
}

func (f *File) ensureOpen(method string) error {
	if f.closed {
		return fmt.Errorf("%s() called on closed file", method)
	}
	return nil
}

func (f *File) Read(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("read", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("read() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	if err := f.ensureOpen("read"); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(f.File)
	if err != nil {
		return nil, fmt.Errorf("read() failed: %w", err)
	}
	return object.String(data), nil
}

func (f *File) Close(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("close", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("close() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	if f.closed {
		return object.Nil, nil
	}
	if err := f.File.Close(); err != nil {
		return nil, fmt.Errorf("close() failed: %w", err)
	}
	f.closed = true
	return object.Nil, nil
}

func (f *File) Stat(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("stat", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("stat() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	if err := f.ensureOpen("stat"); err != nil {
		return nil, err
	}

	info, err := f.File.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat() failed: %w", err)
	}
	return NewFileInfo(info), nil
}

func (f *File) Repr() string {
	return fmt.Sprintf("fs.File(%q)", f.Name)
}

func (f *File) String() string {
	return fmt.Sprintf("<file %s>", f.Name)
}

func (f *File) Bool() bool {
	return !f.closed
}

func (f *File) Compare(object.Object) (int, error) {
	return 0, fmt.Errorf("cannot compare File")
}

func (f *File) Add(object.Object) (object.Object, error) { return nil, fmt.Errorf("cannot add File") }
func (f *File) Minus(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot subtract File")
}
func (f *File) Multiply(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot multiply File")
}
func (f *File) Divide(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot divide File")
}
func (f *File) And(other object.Object) (object.Object, error) {
	return object.Bool(f.Bool() && other.Bool()), nil
}
func (f *File) Or(other object.Object) (object.Object, error) {
	return object.Bool(f.Bool() || other.Bool()), nil
}
func (f *File) Not() (object.Object, error) { return object.Bool(!f.Bool()), nil }
func (f *File) Iter() ([]object.Object, error) {
	return nil, fmt.Errorf("File does not support iteration")
}
func (f *File) Index(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("File is not indexable")
}

func (f *File) GetAttr(name string) (object.Object, error) {
	switch name {
	case "name":
		return object.String(f.Name), nil
	case "closed":
		return object.Bool(f.closed), nil
	case "read":
		return &object.Function{Name: "read", Fn: f.Read}, nil
	case "stat":
		return &object.Function{Name: "stat", Fn: f.Stat}, nil
	case "close":
		return &object.Function{Name: "close", Fn: f.Close}, nil
	default:
		return nil, fmt.Errorf("File has no attribute '%s'", name)
	}
}

var _ object.Object = (*File)(nil)
