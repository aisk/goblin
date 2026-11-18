package archive

import (
	"errors"
	"io"
	stdfs "io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/object"
)

// Sink is the format side of a Writer: it stores one entry. Errors it
// returns are raw; the Adder reports them.
type Sink interface {
	WriteDir(name string, info stdfs.FileInfo) error
	WriteFile(name string, info stdfs.FileInfo, content io.Reader) error
	WriteLink(name string, info stdfs.FileInfo, target string) error
}

// Adder holds what both Writers share: the names already stored, and the
// walk that add(path) does over a file or a tree.
type Adder struct {
	Sink Sink
	// Own describes the archive's own file, passed over when it lies
	// inside an added directory; nil over a stream.
	Own stdfs.FileInfo
	// WriteError reports a failure writing the archive.
	WriteError func(fn string, err error) error
	seen       map[string]bool
}

// Claim records name as stored, refusing one already in the archive.
func (a *Adder) Claim(fn, name string) error {
	if a.seen == nil {
		a.seen = map[string]bool{}
	}
	if a.seen[name] {
		return object.NewValueError("%s() %s is already in the archive", fn, Quote(name))
	}
	a.seen[name] = true
	return nil
}

// Add stores what path names on disk under name, or under the last
// component of path when no name is given. A directory brings everything
// below it, names sorted within each directory; a symbolic link is stored
// as a link and never followed.
func (a *Adder) Add(fn, path, given string, hasName bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return filestream.Fail(fn, "cannot add", path, err)
	}
	if !hasName {
		base := filepath.Base(path)
		if base == "." || base == ".." || base == string(filepath.Separator) {
			return object.NewValueError("%s() %s has no name; pass name=", fn, Quote(path))
		}
		given = base
	}
	name, err := WriterName(fn, given)
	if err != nil {
		return err
	}
	return a.entry(fn, path, name, info)
}

func (a *Adder) entry(fn, path, name string, info stdfs.FileInfo) error {
	mode := info.Mode()
	switch {
	case mode.IsRegular():
		if err := a.Claim(fn, name); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return filestream.Fail(fn, "cannot read", path, err)
		}
		defer file.Close()
		if err := a.Sink.WriteFile(name, info, readSide{file}); err != nil {
			var readErr readFailure
			if errors.As(err, &readErr) {
				return filestream.Fail(fn, "cannot read", path, readErr.err)
			}
			return a.WriteError(fn, err)
		}
		return nil
	case mode&stdfs.ModeSymlink != 0:
		if err := a.Claim(fn, name); err != nil {
			return err
		}
		target, err := os.Readlink(path)
		if err != nil {
			return filestream.Fail(fn, "cannot read", path, err)
		}
		if err := a.Sink.WriteLink(name, info, filepath.ToSlash(target)); err != nil {
			return a.WriteError(fn, err)
		}
		return nil
	case mode.IsDir():
		if err := a.Claim(fn, name); err != nil {
			return err
		}
		if err := a.Sink.WriteDir(name, info); err != nil {
			return a.WriteError(fn, err)
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return filestream.Fail(fn, "cannot read", path, err)
		}
		for _, entry := range entries {
			child := filepath.Join(path, entry.Name())
			childInfo, err := os.Lstat(child)
			if err != nil {
				return filestream.Fail(fn, "cannot add", child, err)
			}
			if a.Own != nil && os.SameFile(childInfo, a.Own) {
				continue
			}
			childName := name + "/" + filepath.ToSlash(entry.Name())
			if !SafeName(childName) {
				return unusableName(fn, childName)
			}
			if err := a.entry(fn, child, childName, childInfo); err != nil {
				return err
			}
		}
		return nil
	}
	return object.NewValueError("%s() %s is not a file, directory or link", fn, Quote(path))
}

// ModeArg checks the mode= of add_data.
func ModeArg(fn string, mode object.Integer) error {
	if mode < 0 || mode > 0o777 {
		return object.RangeArgumentError(fn, "mode", "0", "0o777", OctalText(int64(mode)))
	}
	return nil
}

// Closed is the ValueError for a call on a closed Reader or Writer.
func Closed(fn string) error {
	return object.NewValueError("%s() called on closed archive", fn)
}

// OctalText writes a mode as Goblin's octal literal for it: 0o755, -0o1.
func OctalText(mode int64) string {
	if mode < 0 {
		return "-0o" + strconv.FormatInt(-mode, 8)
	}
	return "0o" + strconv.FormatInt(mode, 8)
}
