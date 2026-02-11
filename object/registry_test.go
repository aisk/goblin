package object

import (
	"errors"
	"testing"
)

func TestRegistry_LoadFirstTime(t *testing.T) {
	r := NewRegistry()
	expected := Object(Integer(42))

	result, err := r.Load("math", func() (Object, error) {
		return expected, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != expected {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestRegistry_LoadCached(t *testing.T) {
	r := NewRegistry()
	expected := Object(Integer(42))
	callCount := 0

	executor := func() (Object, error) {
		callCount++
		return expected, nil
	}

	r.Load("math", executor)
	result, err := r.Load("math", executor)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != expected {
		t.Fatalf("expected %v, got %v", expected, result)
	}
	if callCount != 1 {
		t.Fatalf("executor should be called once, got %d", callCount)
	}
}

func TestRegistry_LoadDifferentPaths(t *testing.T) {
	r := NewRegistry()
	modA := Object(Integer(1))
	modB := Object(Integer(2))

	resultA, _ := r.Load("a", func() (Object, error) { return modA, nil })
	resultB, _ := r.Load("b", func() (Object, error) { return modB, nil })

	if resultA != modA {
		t.Fatalf("expected modA, got %v", resultA)
	}
	if resultB != modB {
		t.Fatalf("expected modB, got %v", resultB)
	}
}

func TestRegistry_LoadError(t *testing.T) {
	r := NewRegistry()
	expectedErr := errors.New("exec failed")

	result, err := r.Load("bad", func() (Object, error) {
		return nil, expectedErr
	})

	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}

	// Verify it was not cached — next call should execute again
	mod, ok := r.Get("bad")
	if ok || mod != nil {
		t.Fatalf("errored module should not be cached")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	_, ok := r.Get("missing")
	if ok {
		t.Fatal("expected false for missing module")
	}

	expected := Object(Integer(10))
	r.Load("exists", func() (Object, error) { return expected, nil })

	mod, ok := r.Get("exists")
	if !ok {
		t.Fatal("expected true for loaded module")
	}
	if mod != expected {
		t.Fatalf("expected %v, got %v", expected, mod)
	}
}
