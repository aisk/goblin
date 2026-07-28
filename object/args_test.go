package object

import (
	"strings"
	"testing"
)

func TestBindArgumentsSuccess(t *testing.T) {
	bound, err := BindArguments("f", []string{"a", "b"}, nil, "", "", CallArgs{
		Positional: Args{Integer(1), Integer(2)},
	})
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
	_, err := BindArguments("f", []string{"a", "b"}, nil, "", "", CallArgs{
		Positional: Args{Integer(1)},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "missing required positional argument: 'b'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBindArgumentsTooManyArgs(t *testing.T) {
	_, err := BindArguments("f", []string{"a"}, nil, "", "", CallArgs{
		Positional: Args{Integer(1), Integer(2)},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "takes 1 positional arguments, got 2") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBindArgumentsKeyword(t *testing.T) {
	bound, err := BindArguments("f", []string{"a", "b"}, nil, "", "", CallArgs{
		Positional: Args{Integer(1)},
		Keyword: Kwargs{
			"b": Integer(2),
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if bound["a"] != Integer(1) || bound["b"] != Integer(2) {
		t.Fatalf("unexpected bound values: %#v", bound)
	}
}

func TestBindArgumentsDuplicateValue(t *testing.T) {
	_, err := BindArguments("f", []string{"a"}, nil, "", "", CallArgs{
		Positional: Args{Integer(1)},
		Keyword: Kwargs{
			"a": Integer(2),
		},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "got multiple values for argument 'a'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBindArgumentsUnexpectedKeyword(t *testing.T) {
	_, err := BindArguments("f", []string{"a"}, nil, "", "", CallArgs{
		Keyword: Kwargs{
			"x": Integer(1),
		},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected keyword argument 'x'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBindArgumentsVarArgsAndKwArgs(t *testing.T) {
	bound, err := BindArguments("f", []string{"a"}, nil, "args", "kwargs", CallArgs{
		Positional: Args{Integer(1), Integer(2), Integer(3)},
		Keyword: Kwargs{
			"x": Integer(4),
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	args, ok := bound["args"].(*List)
	if !ok || len(args.Elements) != 2 {
		t.Fatalf("expected args list, got %#v", bound["args"])
	}
	kwargs, ok := bound["kwargs"].(*Dict)
	if !ok {
		t.Fatalf("expected kwargs dict, got %#v", bound["kwargs"])
	}
	if _, ok, _ := kwargs.Get(String("x")); !ok {
		t.Fatalf("expected kwargs to contain key x")
	}
}

func TestBindArgumentsDefaultUsed(t *testing.T) {
	defaults := []ParamDefault{nil, func() (Object, error) { return Integer(42), nil }}
	bound, err := BindArguments("f", []string{"a", "b"}, defaults, "", "", CallArgs{
		Positional: Args{Integer(1)},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if bound["a"] != Integer(1) || bound["b"] != Integer(42) {
		t.Fatalf("unexpected bound values: %#v", bound)
	}
}

func TestBindArgumentsDefaultOverridden(t *testing.T) {
	called := false
	defaults := []ParamDefault{nil, func() (Object, error) {
		called = true
		return Integer(42), nil
	}}
	bound, err := BindArguments("f", []string{"a", "b"}, defaults, "", "", CallArgs{
		Positional: Args{Integer(1)},
		Keyword: Kwargs{
			"b": Integer(2),
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if bound["b"] != Integer(2) {
		t.Fatalf("unexpected bound values: %#v", bound)
	}
	if called {
		t.Fatalf("default must not be evaluated when the argument is supplied")
	}
}

func TestBindArgumentsDefaultError(t *testing.T) {
	defaults := []ParamDefault{func() (Object, error) { return nil, NewTypeError("boom") }}
	_, err := BindArguments("f", []string{"a"}, defaults, "", "", CallArgs{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBindArgumentsMissingRequiredWithDefaults(t *testing.T) {
	defaults := []ParamDefault{nil, func() (Object, error) { return Integer(42), nil }}
	_, err := BindArguments("f", []string{"a", "b"}, defaults, "", "", CallArgs{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "missing required positional argument: 'a'") {
		t.Fatalf("unexpected error: %v", err)
	}
}
