package object

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestWithFramePreservesTypeAndFormatsTraceback(t *testing.T) {
	base := NewZeroDivisionError("division by zero")
	err := WithFrame(base, Frame{Module: "math", Function: "divide", File: "calc.goblin", Line: 4, Column: 9})
	err = WithFrame(err, Frame{Module: "main", Function: "main", File: "main.goblin", Line: 8, Column: 3})

	if !errors.Is(err, ZeroDivisionError) {
		t.Fatal("framed error no longer matches ZeroDivisionError")
	}
	short := fmt.Sprintf("%v", err)
	if short != "division by zero" {
		t.Fatalf("short error = %q", short)
	}
	trace := fmt.Sprintf("%+v", err)
	for _, want := range []string{
		"Traceback (most recent call last):",
		"at main (main.goblin:8:3)",
		"at divide [math] (calc.goblin:4:9)",
		"division by zero",
	} {
		if !strings.Contains(trace, want) {
			t.Fatalf("traceback missing %q:\n%s", want, trace)
		}
	}
}
