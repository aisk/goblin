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

func TestArgParserOptionalAnyDistinguishesOmittedAndNil(t *testing.T) {
	p := NewArgParser("f", CallArgs{})
	if value, ok := p.OptionalAny("value"); ok || value != nil {
		t.Fatalf("omitted OptionalAny = (%v, %v), want (nil, false)", value, ok)
	}

	p = NewArgParser("f", CallArgs{Positional: Args{Nil}})
	if value, ok := p.OptionalAny("value"); !ok || value != Nil {
		t.Fatalf("explicit nil OptionalAny = (%v, %v), want (Nil, true)", value, ok)
	}
}

func TestArgParserKeywordOnly(t *testing.T) {
	p := NewArgParser("f", CallArgs{Keyword: Kwargs{"a": Integer(1), "b": Integer(2)}})
	a, b := p.Int("a"), p.Int("b")
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if a != 1 || b != 2 {
		t.Fatalf("unexpected values: a=%v b=%v", a, b)
	}
}

func TestArgParserDuplicateArgument(t *testing.T) {
	// f("x", sep=",") where the first accessor is sep: the positional slot
	// already binds sep, so the keyword is a duplicate, not a value for the
	// next parameter.
	p := NewArgParser("split", CallArgs{
		Positional: Args{String("x")},
		Keyword:    Kwargs{"sep": String(",")},
	})
	p.Str("sep")
	p.IntOr("count", -1)
	err := p.Finish()
	if err == nil || !strings.Contains(err.Error(), "split() got multiple values for argument 'sep'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArgParserDuplicateArgumentLaterParam(t *testing.T) {
	// f(1, 2, b=9): the second positional slot already binds b.
	p := NewArgParser("f", CallArgs{
		Positional: Args{Integer(1), Integer(2)},
		Keyword:    Kwargs{"b": Integer(9)},
	})
	p.Int("a")
	p.Int("b")
	err := p.Finish()
	if err == nil || !strings.Contains(err.Error(), "f() got multiple values for argument 'b'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArgParserKeywordAfterPositionalNoConflict(t *testing.T) {
	// f(1, b=2): the positional slot binds a, the keyword binds b — no conflict.
	p := NewArgParser("f", CallArgs{
		Positional: Args{Integer(1)},
		Keyword:    Kwargs{"b": Integer(2)},
	})
	a := p.Int("a")
	b := p.IntOr("b", -1)
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if a != 1 || b != 2 {
		t.Fatalf("unexpected values: a=%v b=%v", a, b)
	}
}

func TestArgParserRestThenKeywordNoConflict(t *testing.T) {
	// f(1, 2, 3, key=9): the variadic tail belongs to Rest, so a keyword-only
	// accessor after Rest must not report a duplicate.
	p := NewArgParser("f", CallArgs{
		Positional: Args{Integer(1), Integer(2), Integer(3)},
		Keyword:    Kwargs{"key": Integer(9)},
	})
	first := p.Int("first")
	rest := p.Rest()
	key := p.IntOr("key", -1)
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if first != 1 || len(rest) != 2 || key != 9 {
		t.Fatalf("unexpected values: first=%v rest=%v key=%v", first, rest, key)
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

func TestArgParserNumberAcceptsIntAndFloat(t *testing.T) {
	// Integer positional flows through Number and preserves its type.
	p := NewArgParser("f", CallArgs{Positional: Args{Integer(7)}})
	v := p.Number("n")
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if i, ok := v.(Integer); !ok || i != 7 {
		t.Fatalf("expected Integer(7), got %#v", v)
	}

	// Float works too.
	p = NewArgParser("f", CallArgs{Positional: Args{Float(1.25)}})
	v = p.Number("n")
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if f, ok := v.(Float); !ok || f != 1.25 {
		t.Fatalf("expected Float(1.25), got %#v", v)
	}
}

func TestArgParserNumberRejectsNonNumeric(t *testing.T) {
	p := NewArgParser("f", CallArgs{Positional: Args{String("x")}})
	p.Number("n")
	err := p.Finish()
	if err == nil || !strings.Contains(err.Error(), "argument 'n' must be number") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArgParserFloat64Coerces(t *testing.T) {
	p := NewArgParser("f", CallArgs{Positional: Args{Integer(3)}})
	got := p.Float64("x")
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != 3.0 {
		t.Fatalf("expected 3.0, got %v", got)
	}

	p = NewArgParser("f", CallArgs{Positional: Args{Float(2.5)}})
	got = p.Float64("x")
	if err := p.Finish(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != 2.5 {
		t.Fatalf("expected 2.5, got %v", got)
	}
}
