package object

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"testing"
)

// ErrorKind matches net.Error structurally so the runtime does not link the
// net package; this pins that the classification is unchanged.
func TestErrorKindClassifiesNativeErrors(t *testing.T) {
	_, notExist := os.Open("/definitely/not/here")
	_, refused := net.Dial("tcp", "127.0.0.1:1")
	timeout := &net.DNSError{Err: "timeout", IsTimeout: true}
	cases := []struct {
		name string
		err  error
		want *Error
	}{
		{"not exist", notExist, NotExistError},
		{"exist", fmt.Errorf("wrapped: %w", fs.ErrExist), ExistError},
		{"permission", fs.ErrPermission, PermissionError},
		{"network", refused, NetworkError},
		{"network timeout", timeout, TimeoutError},
		{"deadline", context.DeadlineExceeded, TimeoutError},
		{"other", fmt.Errorf("boom"), IOError},
	}
	for _, c := range cases {
		if c.err == nil {
			t.Fatalf("%s: expected an error to classify", c.name)
		}
		if got := ErrorKind(c.err, IOError); got != c.want {
			t.Errorf("%s: ErrorKind(%v) = %v, want %v", c.name, c.err, got, c.want)
		}
	}
}
