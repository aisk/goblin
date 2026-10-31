package object

import (
	"errors"
	"testing"
)

func recurse(n int) (int, error) {
	if err := CheckStack(); err != nil {
		return n, err
	}
	var pad [256]byte
	pad[n%256] = byte(n)
	m, err := recurse(n + 1)
	return m + int(pad[n%256]&0), err
}

func TestCheckStackStopsRunawayRecursion(t *testing.T) {
	if stackUsed() == 0 {
		t.Skip("stack checking is not available on this architecture")
	}
	check := func() {
		n, err := recurse(0)
		if !errors.Is(err, RecursionError) {
			t.Errorf("err = %v, want RecursionError", err)
		}
		if n < 10000 {
			t.Errorf("stopped after only %d levels", n)
		}
	}
	check()
	// Every goroutine measures its own stack.
	done := make(chan struct{})
	go func() {
		defer close(done)
		check()
	}()
	<-done
	if used := stackUsed(); used > 1<<20 {
		t.Errorf("stackUsed() = %d at shallow depth", used)
	}
}
