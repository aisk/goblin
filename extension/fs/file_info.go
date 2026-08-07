package fs

import (
	"fmt"
	stdfs "io/fs"

	"github.com/aisk/goblin/object"
)

type FileInfo struct {
	object.OpaqueBase
	Info stdfs.FileInfo
}

func NewFileInfo(info stdfs.FileInfo) *FileInfo {
	return &FileInfo{OpaqueBase: object.MakeOpaqueBase("FileInfo"), Info: info}
}

func (f *FileInfo) String() string {
	return fmt.Sprintf("<file_info %s>", f.Info.Name())
}

func (f *FileInfo) ToString() (string, error) { return f.String(), nil }

func (f *FileInfo) Equals(other object.Object) (bool, error) {
	v, ok := other.(*FileInfo)
	return ok && f == v, nil
}

func (f *FileInfo) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(f), nil
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
		return nil, object.NewAttributeError("FileInfo has no attribute '%s'", name)
	}
}

func (f *FileInfo) Attributes() []string {
	return []string{"attributes", "name", "size", "is_dir", "mode", "mod_time"}
}

var _ object.Object = (*FileInfo)(nil)
