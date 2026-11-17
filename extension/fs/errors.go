package fs

import (
	"errors"
	stdfs "io/fs"
	"os"
	"syscall"

	"github.com/aisk/goblin/object"
)

// osCause strips the operation and path Go adds to an OS error, since the
// message wrapping it already names both.
func osCause(err error) error {
	var pe *stdfs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return le.Err
	}
	var se *os.SyscallError
	if errors.As(err, &se) {
		return se.Err
	}
	return err
}

// fail wraps an OS error as `fn() what "path": cause`, keeping its kind
// (NotExistError, ExistError, PermissionError) and falling back to IOError.
func fail(fn, what, path string, err error) error {
	var goblinErr *object.Error
	if errors.As(err, &goblinErr) && !isOSError(err) {
		return err
	}
	return object.WrapNativeError(object.IOError, fn+"() "+what+" "+quote(path), osCause(err))
}

func isOSError(err error) bool {
	var pe *stdfs.PathError
	var le *os.LinkError
	var se *os.SyscallError
	var errno syscall.Errno
	return errors.As(err, &pe) || errors.As(err, &le) || errors.As(err, &se) || errors.As(err, &errno)
}

// kindError builds an error of a given kind about path, with no OS cause.
func kindError(kind *object.Error, fn, format, path string) error {
	return &object.Error{Value: fn + "() " + quote(path) + " " + format, Wrapped: kind}
}

// notThere reports whether err means that nothing is at the path: it does
// not exist, or a component of it is not a directory.
func notThere(err error) bool {
	return errors.Is(err, stdfs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}
