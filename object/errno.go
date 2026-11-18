//go:build !plan9

package object

import "syscall"

// errnoKind is the kind of an operating-system error number that no network
// error wraps: a network condition reported by a plain system call (a
// refused connection, a broken pipe), or fallback.
func errnoKind(errno syscall.Errno, fallback *Error) *Error {
	switch errno {
	case syscall.ETIMEDOUT:
		return TimeoutError
	case syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.EHOSTUNREACH, syscall.ENETUNREACH, syscall.EPIPE:
		return NetworkError
	}
	return fallback
}
