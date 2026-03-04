package object

import (
	"strings"
	"testing"
)

func TestBindArgumentsPartialAllowsMissing(t *testing.T) {
	bound, err := BindArgumentsPartial("f", []string{"a", "b", "c"}, Args{Integer(1)}, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(bound) != 1 {
		t.Fatalf("expected 1 bound value, got %d", len(bound))
	}
	if _, ok := bound["a"]; !ok {
		t.Fatalf("expected 'a' to be bound")
	}
	if _, ok := bound["b"]; ok {
		t.Fatalf("expected 'b' to be missing")
	}
}

func TestBindArgumentsPartialUnexpectedKeyword(t *testing.T) {
	_, err := BindArgumentsPartial("f", []string{"a"}, nil, KwArgs{"b": Integer(1)})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected keyword argument 'b'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBindArgumentsPartialMultipleValues(t *testing.T) {
	_, err := BindArgumentsPartial("f", []string{"a"}, Args{Integer(1)}, KwArgs{"a": Integer(2)})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "multiple values for argument 'a'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBindArgumentsStillRequiresAllArguments(t *testing.T) {
	_, err := BindArguments("f", []string{"a", "b"}, Args{Integer(1)}, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "missing required argument 'b'") {
		t.Fatalf("unexpected error: %v", err)
	}
}
