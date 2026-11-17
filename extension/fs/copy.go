package fs

import (
	"errors"
	"io"
	stdfs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/aisk/goblin/object"
)

func srcDstArgs(fn string, args object.CallArgs) (string, string, error) {
	p := object.NewArgParser(fn, args)
	src := p.Path("src")
	dst := p.Path("dst")
	if err := p.Finish(); err != nil {
		return "", "", err
	}
	return src, dst, nil
}

// failPair is fail for an operation from src to dst.
func failPair(fn, verb, src, dst string, err error) error {
	return fail(fn, "cannot "+verb+" "+quote(src)+" to", dst, err)
}

func copyPath(args object.CallArgs) (object.Object, error) {
	src, dst, err := srcDstArgs("copy", args)
	if err != nil {
		return nil, err
	}
	if err := copyAny("copy", src, dst); err != nil {
		return nil, err
	}
	return object.Nil, nil
}

// copyAny copies a file onto dst, or a directory as a new tree at dst.
func copyAny(fn, src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fail(fn, "cannot copy", src, err)
	}
	dstInfo, dstErr := os.Lstat(dst)
	if info.IsDir() {
		if dstErr == nil {
			return kindError(object.ExistError, fn, "already exists", dst)
		}
		if within(src, dst) {
			return pathError(object.IOError, dst, "%s() cannot copy %s into itself (%s)", fn, quote(src), quote(dst))
		}
		return copyTree(fn, src, dst, info)
	}
	if dstErr == nil {
		if dstInfo.IsDir() {
			return kindError(object.ExistError, fn, "is a directory", dst)
		}
		if target, err := os.Stat(dst); err == nil && os.SameFile(info, target) {
			return pathError(object.IOError, dst, "%s() %s and %s are the same file", fn, quote(src), quote(dst))
		}
	}
	return copyFile(fn, src, dst, info.Mode().Perm())
}

func copyFile(fn, src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fail(fn, "cannot copy", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return failPair(fn, "copy", src, dst, err)
	}
	_, err = io.Copy(out, in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(dst, perm)
	}
	if err != nil {
		return failPair(fn, "copy", src, dst, err)
	}
	return nil
}

// within reports whether the not yet existing path dst is the directory src
// or lies below it, once both are made absolute and their symbolic links
// resolved. A copy there would descend into what it is writing.
func within(src, dst string) bool {
	from, err := realPath(src)
	if err != nil {
		return false
	}
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return false
	}
	parent, err := realPath(filepath.Dir(absDst))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(from, filepath.Join(parent, filepath.Base(absDst)))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func realPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// copyTree copies the directory src to the new path dst. Symbolic links are
// copied as links.
func copyTree(fn, src, dst string, info stdfs.FileInfo) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fail(fn, "cannot read", src, err)
	}
	if err := os.Mkdir(dst, 0o700); err != nil {
		return failPair(fn, "copy", src, dst, err)
	}
	for _, entry := range entries {
		from, to := filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())
		switch {
		case entry.Type()&stdfs.ModeSymlink != 0:
			target, err := os.Readlink(from)
			if err == nil {
				err = os.Symlink(target, to)
			}
			if err != nil {
				return failPair(fn, "copy", from, to, err)
			}
		case entry.IsDir():
			sub, err := entry.Info()
			if err != nil {
				return fail(fn, "cannot copy", from, err)
			}
			if err := copyTree(fn, from, to, sub); err != nil {
				return err
			}
		case entry.Type().IsRegular():
			sub, err := entry.Info()
			if err != nil {
				return fail(fn, "cannot copy", from, err)
			}
			if err := copyFile(fn, from, to, sub.Mode().Perm()); err != nil {
				return err
			}
		default:
			return kindError(object.IOError, fn, "is not a regular file, directory or symbolic link", from)
		}
	}
	if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
		return failPair(fn, "copy", src, dst, err)
	}
	return nil
}

func move(args object.CallArgs) (object.Object, error) {
	src, dst, err := srcDstArgs("move", args)
	if err != nil {
		return nil, err
	}
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return nil, fail("move", "cannot move", src, err)
	}
	if info, err := os.Lstat(dst); err == nil && info.IsDir() {
		return nil, kindError(object.ExistError, "move", "is a directory", dst)
	}
	err = os.Rename(src, dst)
	if err == nil {
		return object.Nil, nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return nil, failPair("move", "move", src, dst, err)
	}
	if err := moveAcross(src, dst, srcInfo); err != nil {
		return nil, err
	}
	return object.Nil, nil
}

// moveAcross moves src to dst on another file system: it copies, then
// removes the original. A symbolic link is moved as a link, as rename moves
// it, rather than as a copy of what it points to.
func moveAcross(src, dst string, srcInfo stdfs.FileInfo) error {
	if srcInfo.Mode()&stdfs.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return fail("move", "cannot move", src, err)
		}
		// Like rename, replace a file already at dst.
		if err := os.Remove(dst); err != nil && !notThere(err) {
			return failPair("move", "move", src, dst, err)
		}
		if err := os.Symlink(target, dst); err != nil {
			return failPair("move", "move", src, dst, err)
		}
	} else if err := copyAny("move", src, dst); err != nil {
		return err
	}
	if err := os.RemoveAll(src); err != nil {
		return fail("move", "cannot remove", src, err)
	}
	return nil
}
