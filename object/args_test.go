package object

import (
	"strings"
	"testing"
)

func TestBindArgumentsSuccess(t *testing.T) {
	bound, err := BindArguments("f", []string{"a", "b"}, Args{Integer(1), Integer(2)})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(bound) != 2 {
		t.Fatalf("expected 2 bound values, got %d", len(bound))
	}
	if _, ok := bound["a"]; !ok {
		t.Fatalf("expected 'a' to be bound")
	}
	if _, ok := bound["b"]; !ok {
		t.Fatalf("expected 'b' to be bound")
	}
}

func TestBindArgumentsTooFewArgs(t *testing.T) {
	_, err := BindArguments("f", []string{"a", "b"}, Args{Integer(1)})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "takes 2 positional arguments, got 1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBindArgumentsTooManyArgs(t *testing.T) {
	_, err := BindArguments("f", []string{"a"}, Args{Integer(1), Integer(2)})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "takes 1 positional arguments, got 2") {
		t.Fatalf("unexpected error: %v", err)
	}
}
