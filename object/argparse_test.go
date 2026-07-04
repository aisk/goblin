package object

import (
	"strings"
	"testing"
)

func TestArgParserPositional(t *testing.T) {
	p := NewArgParser("f", CallArgs{Positional: Args{Integer(1), Integer(2)}})
	a, b := p.Int("a"), p.Int("b")
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if a != 1 || b != 2 {
		t.Fatalf("unexpected values: a=%v b=%v", a, b)
	}
}

func TestArgParserKeywordPrecedence(t *testing.T) {
	p := NewArgParser("f", CallArgs{
		Positional: Args{Integer(1)},
		Keyword:    Kwargs{"b": Integer(2)},
	})
	a, b := p.Int("a"), p.Int("b")
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if a != 1 || b != 2 {
		t.Fatalf("unexpected values: a=%v b=%v", a, b)
	}
}

func TestArgParserMissingArgument(t *testing.T) {
	p := NewArgParser("f", CallArgs{Positional: Args{Integer(1)}})
	p.Int("a")
	p.Int("b")
	err := p.Finish()
	if err == nil || !strings.Contains(err.Error(), "missing required argument: 'b'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArgParserTypeMismatch(t *testing.T) {
	p := NewArgParser("f", CallArgs{Positional: Args{String("x")}})
	p.Int("a")
	err := p.Finish()
	if err == nil || !strings.Contains(err.Error(), "argument 'a' must be int") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArgParserTooManyPositional(t *testing.T) {
	p := NewArgParser("f", CallArgs{Positional: Args{Integer(1), Integer(2)}})
	p.Int("a")
	err := p.Finish()
	if err == nil || !strings.Contains(err.Error(), "takes 1 positional arguments, got 2") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArgParserUnexpectedKeyword(t *testing.T) {
	p := NewArgParser("f", CallArgs{
		Positional: Args{Integer(1)},
		Keyword:    Kwargs{"x": Integer(9)},
	})
	p.Int("a")
	err := p.Finish()
	if err == nil || !strings.Contains(err.Error(), "unexpected keyword argument 'x'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArgParserOptionalDefault(t *testing.T) {
	p := NewArgParser("f", CallArgs{Positional: Args{Integer(1)}})
	a := p.Int("a")
	step := p.IntOr("step", 10)
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if a != 1 || step != 10 {
		t.Fatalf("unexpected values: a=%v step=%v", a, step)
	}
}

func TestArgParserRest(t *testing.T) {
	p := NewArgParser("f", CallArgs{Positional: Args{Integer(1), Integer(2), Integer(3)}})
	first := p.Int("first")
	rest := p.Rest()
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if first != 1 || len(rest) != 2 {
		t.Fatalf("unexpected values: first=%v rest=%v", first, rest)
	}
}

func TestArgParserFunc(t *testing.T) {
	fn := &Function{Name: "g"}
	p := NewArgParser("f", CallArgs{Positional: Args{fn}})
	got := p.Func("fn")
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != fn {
		t.Fatalf("expected the function back, got %#v", got)
	}
}

func TestArgParserFuncTypeMismatch(t *testing.T) {
	p := NewArgParser("f", CallArgs{Positional: Args{Integer(1)}})
	p.Func("fn")
	err := p.Finish()
	if err == nil || !strings.Contains(err.Error(), "argument 'fn' must be function") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArgParserTypedAccessors(t *testing.T) {
	p := NewArgParser("f", CallArgs{Positional: Args{Float(1.5), String("s"), True}})
	f := p.Float("f")
	s := p.Str("s")
	b := p.Bool("b")
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if f != 1.5 || s != "s" || b != true {
		t.Fatalf("unexpected values: f=%v s=%v b=%v", f, s, b)
	}
}
