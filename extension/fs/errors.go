package fs

import (
	"errors"
	stdfs "io/fs"
	"syscall"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

// osCause strips the operation and path Go adds to an OS error, since the
// message wrapping it already names both.
func osCause(err error) error { return filestream.OSCause(err) }

// fail wraps an OS error as `fn() what "path": cause`, keeping its kind
// (NotExistError, ExistError, PermissionError) and falling back to IOError;
// the error carries path.
func fail(fn, what, path string, err error) error {
	return filestream.Fail(fn, what, path, err)
}

// kindError builds an error of a given kind about path, with no OS cause;
// the error carries path.
func kindError(kind *object.Error, fn, format, path string) error {
	return pathError(kind, path, "%s() %s %s", fn, quote(path), format)
}

// pathError builds an error of kind with a formatted message about path and
// attaches path as data.
func pathError(kind *object.Error, path string, format string, a ...any) *object.Error {
	return object.NewKindError(kind, format, a...).With("path", pathvalue.NewPath(path))
}

// notThere reports whether err means that nothing is at the path: it does
// not exist, or a component of it is not a directory.
func notThere(err error) bool {
	return errors.Is(err, stdfs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// quote is how messages name a path.
func quote(path string) string { return filestream.Quote(path) }
