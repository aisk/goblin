// Package fs is the fs module: files, directories and path names. fs
// functions touch the disk; a Path only looks at the name.
package fs

import (
	"errors"
	"io"
	stdfs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return object.NewModule("fs", map[string]object.Object{
		"Path":     pathvalue.Type.Function,
		"File":     filestream.Type.Function,
		"FileInfo": fileInfoType.Function,
		"Lines":    filestream.LinesType.Function,
		"Walk":     walkType.Function,
	}, object.Funcs{
		"read":       read,
		"read_bytes": readBytes,
		"read_lines": readLines,
		"write":      write,
		"open":       open,
		"exists":     exists,
		"is_file":    isFile,
		"is_dir":     isDir,
		"stat":       stat,
		"read_dir":   readDir,
		"walk":       walk,
		"glob":       glob,
		"mkdir":      mkdir,
		"mkdir_temp": mkdirTemp,
		"remove":     remove,
		"copy":       copyPath,
		"move":       move,
		"chmod":      chmod,
		"resolve":    resolve,
		"cwd":        cwd,
		"home":       home,
		"temp_dir":   tempDir,
	}), nil
}

func pathArg(fn string, args object.CallArgs) (string, error) {
	p := object.NewArgParser(fn, args)
	path := p.Path("path")
	if err := p.Finish(); err != nil {
		return "", err
	}
	return path, nil
}

func readAll(fn, path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fail(fn, "cannot open", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fail(fn, "cannot read", path, err)
	}
	return data, nil
}

func read(args object.CallArgs) (object.Object, error) {
	path, err := pathArg("read", args)
	if err != nil {
		return nil, err
	}
	data, err := readAll("read", path)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) {
		return nil, pathError(object.ValueError, path, "read() %s is not valid UTF-8", quote(path))
	}
	return object.String(data), nil
}

func readBytes(args object.CallArgs) (object.Object, error) {
	path, err := pathArg("read_bytes", args)
	if err != nil {
		return nil, err
	}
	data, err := readAll("read_bytes", path)
	if err != nil {
		return nil, err
	}
	return object.NewBytes(data), nil
}

func write(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("write", args)
	path := p.Path("path")
	data := p.Data("data")
	appending := bool(p.BoolOr("append", false))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appending {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	file, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return nil, fail("write", "cannot open", path, err)
	}
	_, err = io.Copy(file, data)
	closeErr := file.Close()
	if err != nil {
		return nil, fail("write", "cannot write", path, err)
	}
	if closeErr != nil {
		return nil, fail("write", "cannot write", path, closeErr)
	}
	return object.Nil, nil
}

func open(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("open", args)
	path := p.Path("path")
	writing := bool(p.BoolOr("write", false))
	appending := bool(p.BoolOr("append", false))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	var file *os.File
	var err error
	switch {
	case appending:
		file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	case writing:
		file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	default:
		file, err = os.Open(path)
	}
	if err != nil {
		return nil, fail("open", "cannot open", path, err)
	}
	return filestream.Open(path, file, writing || appending), nil
}

// statOr stats path following symlinks; a path with nothing there answers
// nil and no error.
func statOr(fn, path string) (stdfs.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		if notThere(err) {
			return nil, nil
		}
		return nil, fail(fn, "cannot stat", path, err)
	}
	return info, nil
}

func exists(args object.CallArgs) (object.Object, error) {
	path, err := pathArg("exists", args)
	if err != nil {
		return nil, err
	}
	info, err := statOr("exists", path)
	if err != nil {
		return nil, err
	}
	return object.Bool(info != nil), nil
}

func isFile(args object.CallArgs) (object.Object, error) {
	path, err := pathArg("is_file", args)
	if err != nil {
		return nil, err
	}
	info, err := statOr("is_file", path)
	if err != nil {
		return nil, err
	}
	return object.Bool(info != nil && info.Mode().IsRegular()), nil
}

func isDir(args object.CallArgs) (object.Object, error) {
	path, err := pathArg("is_dir", args)
	if err != nil {
		return nil, err
	}
	info, err := statOr("is_dir", path)
	if err != nil {
		return nil, err
	}
	return object.Bool(info != nil && info.IsDir()), nil
}

func stat(args object.CallArgs) (object.Object, error) {
	path, err := pathArg("stat", args)
	if err != nil {
		return nil, err
	}
	link, err := os.Lstat(path)
	if err != nil {
		return nil, fail("stat", "cannot stat", path, err)
	}
	isLink := link.Mode()&stdfs.ModeSymlink != 0
	if !isLink {
		return newFileInfo(link, false), nil
	}
	target, err := os.Stat(path)
	if err != nil {
		if notThere(err) {
			// A broken link: describe the link itself.
			return newFileInfo(link, true), nil
		}
		return nil, fail("stat", "cannot stat", path, err)
	}
	return newFileInfo(target, true), nil
}

// dirEntries lists a directory sorted by name, raising IOError when path is
// not a directory.
func dirEntries(fn, path string) ([]os.DirEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fail(fn, "cannot read", path, err)
	}
	if !info.IsDir() {
		return nil, kindError(object.IOError, fn, "is not a directory", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fail(fn, "cannot read", path, err)
	}
	return entries, nil
}

func readDir(args object.CallArgs) (object.Object, error) {
	path, err := pathArg("read_dir", args)
	if err != nil {
		return nil, err
	}
	entries, err := dirEntries("read_dir", path)
	if err != nil {
		return nil, err
	}
	items := make([]object.Object, len(entries))
	for i, entry := range entries {
		items[i] = pathvalue.NewPath(filepath.Join(path, entry.Name()))
	}
	return &object.List{Elements: items}, nil
}

func mkdir(args object.CallArgs) (object.Object, error) {
	path, err := pathArg("mkdir", args)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		if errors.Is(err, syscall.ENOTDIR) || errors.Is(err, stdfs.ErrExist) {
			return nil, pathError(object.ExistError, path, "mkdir() cannot create %s: a file that is not a directory is in the way", quote(path))
		}
		return nil, fail("mkdir", "cannot create", path, err)
	}
	return object.Nil, nil
}

func mkdirTemp(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("mkdir_temp", args)
	prefix := string(p.StrOr("prefix", ""))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if strings.ContainsFunc(prefix, func(r rune) bool { return r < 128 && os.IsPathSeparator(uint8(r)) }) {
		return nil, object.NewValueError("mkdir_temp() argument 'prefix' must not contain a path separator, got %q", prefix)
	}
	dir, err := os.MkdirTemp("", prefix+"*")
	if err != nil {
		return nil, fail("mkdir_temp", "cannot create a directory in", os.TempDir(), err)
	}
	return pathvalue.NewPath(dir), nil
}

func remove(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("remove", args)
	path := p.Path("path")
	recursive := bool(p.BoolOr("recursive", false))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if recursive {
		if err := os.RemoveAll(path); err != nil {
			return nil, fail("remove", "cannot remove", path, err)
		}
		return object.Nil, nil
	}
	err := os.Remove(path)
	if err == nil || notThere(err) {
		return object.Nil, nil
	}
	if info, statErr := os.Lstat(path); statErr == nil && info.IsDir() {
		if entries, readErr := os.ReadDir(path); readErr == nil && len(entries) > 0 {
			return nil, pathError(object.IOError, path, "remove() directory %s is not empty (pass recursive=true)", quote(path))
		}
	}
	return nil, fail("remove", "cannot remove", path, err)
}

func chmod(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("chmod", args)
	path := p.Path("path")
	mode := p.Int("mode")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if mode < 0 || mode > 0o777 {
		return nil, object.NewValueError("chmod() argument 'mode' must be between 0 and 0o777, got 0o%o", int64(mode))
	}
	if err := os.Chmod(path, os.FileMode(mode)); err != nil {
		return nil, fail("chmod", "cannot change the mode of", path, err)
	}
	return object.Nil, nil
}

func resolve(args object.CallArgs) (object.Object, error) {
	path, err := pathArg("resolve", args)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fail("resolve", "cannot resolve", path, err)
	}
	resolved, err := evalExisting(abs)
	if err != nil {
		return nil, fail("resolve", "cannot resolve", path, err)
	}
	return pathvalue.NewPath(resolved), nil
}

// evalExisting resolves the symlinks of the longest prefix of an absolute
// path that exists and appends the rest unchanged.
func evalExisting(abs string) (string, error) {
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	if !notThere(err) {
		return "", err
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return abs, nil
	}
	head, err := evalExisting(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(head, filepath.Base(abs)), nil
}

func cwd(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("cwd", args); err != nil {
		return nil, err
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "cwd() cannot get the working directory", osCause(err))
	}
	return pathvalue.NewPath(dir), nil
}

func home(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("home", args); err != nil {
		return nil, err
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "home() cannot find the home directory", err)
	}
	return pathvalue.NewPath(dir), nil
}

func tempDir(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("temp_dir", args); err != nil {
		return nil, err
	}
	return pathvalue.NewPath(os.TempDir()), nil
}

