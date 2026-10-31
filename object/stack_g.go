//go:build amd64 || arm64

package object

import "unsafe"

// getg returns the running goroutine's runtime g.
func getg() unsafe.Pointer

// gStack mirrors the first field of the runtime's g, the bounds of the
// goroutine's stack. Its position is fixed: the runtime's cgo and assembly
// code rely on stack being at offset 0.
type gStack struct{ lo, hi uintptr }

// stackUsed returns roughly how many bytes of the current goroutine's stack
// are in use.
//
//go:nosplit
func stackUsed() uintptr {
	var marker byte
	return (*gStack)(getg()).hi - uintptr(unsafe.Pointer(&marker))
}
