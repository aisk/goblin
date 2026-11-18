package fs

import (
	stdfs "io/fs"
	"strconv"

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

// String names the kind the properties describe: <FileInfo file size=12>,
// <FileInfo dir>, <FileInfo other>. For a symbolic link the properties
// describe its target, so a link shows its target's kind; only a broken link,
// which describes the link itself, shows as <FileInfo symlink>.
func (f *FileInfo) String() string {
	switch {
	case f.info.Mode().IsRegular():
		return "<FileInfo file size=" + strconv.FormatInt(f.info.Size(), 10) + ">"
	case f.info.IsDir():
		return "<FileInfo dir>"
	case f.symlink:
		return "<FileInfo symlink>"
	}
	return "<FileInfo other>"
}

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
