package fs

import (
	"fmt"
	stdfs "io/fs"

	"github.com/aisk/goblin/object"
)

type FileInfo struct {
	Info stdfs.FileInfo
}

func NewFileInfo(info stdfs.FileInfo) *FileInfo {
	return &FileInfo{Info: info}
}

func (f *FileInfo) Repr() string {
	return fmt.Sprintf("fs.FileInfo(%q)", f.Info.Name())
}

func (f *FileInfo) String() string {
	return fmt.Sprintf("<file_info %s>", f.Info.Name())
}

func (f *FileInfo) Bool() bool {
	return true
}

func (f *FileInfo) Compare(object.Object) (int, error) {
	return 0, fmt.Errorf("cannot compare FileInfo")
}

func (f *FileInfo) Add(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot add FileInfo")
}
func (f *FileInfo) Minus(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot subtract FileInfo")
}
func (f *FileInfo) Multiply(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot multiply FileInfo")
}
func (f *FileInfo) Divide(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot divide FileInfo")
}
func (f *FileInfo) And(other object.Object) (object.Object, error) {
	return object.Bool(f.Bool() && other.Bool()), nil
}
func (f *FileInfo) Or(other object.Object) (object.Object, error) {
	return object.Bool(f.Bool() || other.Bool()), nil
}
func (f *FileInfo) Not() (object.Object, error) { return object.Bool(!f.Bool()), nil }
func (f *FileInfo) Iter() ([]object.Object, error) {
	return nil, fmt.Errorf("FileInfo does not support iteration")
}
func (f *FileInfo) Index(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("FileInfo is not indexable")
}

func (f *FileInfo) GetAttr(name string) (object.Object, error) {
	switch name {
	case "name":
		return object.String(f.Info.Name()), nil
	case "size":
		return object.Integer(f.Info.Size()), nil
	case "is_dir":
		return object.Bool(f.Info.IsDir()), nil
	case "mode":
		return object.String(f.Info.Mode().String()), nil
	case "mod_time":
		return object.String(f.Info.ModTime().Format("2006-01-02T15:04:05Z07:00")), nil
	default:
		return nil, fmt.Errorf("FileInfo has no attribute '%s'", name)
	}
}

var _ object.Object = (*FileInfo)(nil)
