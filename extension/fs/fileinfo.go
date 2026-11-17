package fs

import (
	stdfs "io/fs"

	"github.com/aisk/goblin/extension/internal/timevalue"
	"github.com/aisk/goblin/object"
)

var fileInfoType = object.NewOpaqueConstructor("FileInfo", "fs.stat(path)")

// FileInfo is an immutable snapshot of a directory entry, from fs.stat.
type FileInfo struct {
	object.OpaqueBase
	info    stdfs.FileInfo
	symlink bool
}

func newFileInfo(info stdfs.FileInfo, symlink bool) *FileInfo {
	return &FileInfo{OpaqueBase: object.MakeOpaqueBase("FileInfo"), info: info, symlink: symlink}
}

func (f *FileInfo) Constructor() *object.Function { return fileInfoType.Function }

func (f *FileInfo) String() string            { return "<FileInfo>" }
func (f *FileInfo) ToString() (string, error) { return f.String(), nil }

func (f *FileInfo) GetAttr(name string) (object.Object, error) {
	switch name {
	case "size":
		return object.Integer(f.info.Size()), nil
	case "modified_at":
		return timevalue.NewTime(f.info.ModTime()), nil
	case "is_file":
		return object.Bool(f.info.Mode().IsRegular()), nil
	case "is_dir":
		return object.Bool(f.info.IsDir()), nil
	case "is_symlink":
		return object.Bool(f.symlink), nil
	case "mode":
		return object.Integer(f.info.Mode().Perm()), nil
	}
	return nil, object.NewAttributeError("FileInfo has no attribute '%s'", name)
}

func (f *FileInfo) Attributes() []string {
	return []string{"size", "modified_at", "is_file", "is_dir", "is_symlink", "mode"}
}
