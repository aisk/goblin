package filestream

import (
	"errors"
	stdfs "io/fs"
	"os"
	"syscall"

	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

// OSCause strips the operation and path Go adds to an OS error, since the
// message wrapping it already names both.
func OSCause(err error) error {
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

// Fail wraps an OS error about path as `fn() what "path": cause`, keeping its
// kind (NotExistError, ExistError, PermissionError) and falling back to
// IOError. The error carries path as a Path. An error that is already a
// Goblin error (raised by a readable, or built by an opener) passes through
// unchanged with whatever data it has.
func Fail(fn, what, path string, err error) error {
	var goblinErr *object.Error
	if errors.As(err, &goblinErr) && !isOSError(err) {
		return err
	}
	return object.WrapNativeError(object.IOError, fn+"() "+what+" "+Quote(path), OSCause(err)).
		With("path", pathvalue.NewPath(path))
}

func isOSError(err error) bool {
	var pe *stdfs.PathError
	var le *os.LinkError
	var se *os.SyscallError
	var errno syscall.Errno
	return errors.As(err, &pe) || errors.As(err, &le) || errors.As(err, &se) || errors.As(err, &errno)
}

// Quote is how messages name a path.
func Quote(path string) string { return `"` + path + `"` }

// InvalidLine is the ValueError for a line that is not valid UTF-8: line
// counts from 1, and where names the stream. path, when not empty, is
// attached as data together with the line number.
func InvalidLine(line int, where, path string) *object.Error {
	err := object.NewValueError("line %d of %s is not valid UTF-8", line, where)
	if path != "" {
		err.With("path", pathvalue.NewPath(path))
	}
	return err.With("line", object.Integer(line))
}

// MethodError is an error a reader returns when the message has to name the
// Goblin call it surfaced through (gzip's "read() corrupt gzip data in …").
// A File or Lines asks it for the error to raise; method is "" for a `for`
// loop, which is not a call.
type MethodError interface {
	error
	ForMethod(method string) error
}

// methodError returns the Goblin error for err as raised by method, when err
// carries one.
func methodError(method string, err error) (error, bool) {
	var me MethodError
	if errors.As(err, &me) {
		return me.ForMethod(method), true
	}
	return nil, false
}
