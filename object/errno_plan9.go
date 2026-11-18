package object

import "syscall"

// errnoKind is fallback on Plan 9, whose system calls report errors as text.
func errnoKind(errno syscall.Errno, fallback *Error) *Error {
	return fallback
}
