//go:build !amd64 && !arm64

package object

// stackUsed is unavailable on this architecture, so recursion is not limited
// and runaway recursion still ends in Go's fatal stack overflow.
func stackUsed() uintptr { return 0 }
