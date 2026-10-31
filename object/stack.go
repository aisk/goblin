package object

// Go treats exhausting a goroutine's stack as a fatal error that no recover
// can catch, so unbounded Goblin recursion would kill the whole process. Both
// backends call CheckStack on entry to every Goblin function (compiled
// direct calls every StackCheckInterval levels), turning the runaway into a
// catchable RecursionError while the stack still has plenty of room.

// MaxStack is the stack a goroutine may use before Goblin code raises
// RecursionError. Go's own limit is 1 GB.
const MaxStack = 256 << 20

// StackCheckInterval is how many nested direct calls compiled code makes
// between two CheckStack calls. Interval-1 must stay usable as a mask.
const StackCheckInterval = 1024

// ErrRecursionDepth is the message of every RecursionError.
const ErrRecursionDepth = "maximum recursion depth exceeded"

// StackOK reports whether the current goroutine's stack still has room. It
// is small enough to inline into every function prologue; on false the caller
// raises NewRecursionDepthError.
func StackOK() bool { return stackUsed() <= MaxStack }

// CheckStack is StackOK for Go code that already returns an error.
func CheckStack() error {
	if !StackOK() {
		return NewRecursionDepthError()
	}
	return nil
}

// NewRecursionDepthError is the error raised when StackOK fails, kept out of
// line so the prologues that call it stay small.
//
//go:noinline
func NewRecursionDepthError() error { return NewRecursionError(ErrRecursionDepth) }
