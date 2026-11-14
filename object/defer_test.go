package object

import (
	"errors"
	"reflect"
	"testing"
)

func TestDefersRunLastInFirstOut(t *testing.T) {
	var order []int
	var d Defers
	for i := 0; i < 3; i++ {
		d.Push(func() error {
			order = append(order, i)
			return nil
		})
	}
	if err := d.Run(nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !reflect.DeepEqual(order, []int{2, 1, 0}) {
		t.Errorf("order = %v, want [2 1 0]", order)
	}
}

func TestDefersRunKeepsTheErrorInFlight(t *testing.T) {
	inFlight := NewError("in flight")
	var d Defers
	d.Push(func() error { return nil })
	if err := d.Run(inFlight); !errors.Is(err, inFlight) {
		t.Errorf("Run = %v, want the error in flight", err)
	}
	if err := (Defers)(nil).Run(inFlight); !errors.Is(err, inFlight) {
		t.Errorf("empty Run = %v, want the error in flight", err)
	}
}

func TestDefersRunReplacesTheErrorAndKeepsGoing(t *testing.T) {
	first, second := NewError("first"), NewError("second")
	ran := 0
	var d Defers
	d.Push(func() error { ran++; return second })
	d.Push(func() error { ran++; return nil })
	d.Push(func() error { ran++; return first })
	err := d.Run(NewError("in flight"))
	if ran != 3 {
		t.Errorf("ran %d deferred calls, want 3", ran)
	}
	// The call that runs last decides.
	if !errors.Is(err, second) {
		t.Errorf("Run = %v, want the last error raised", err)
	}
}
