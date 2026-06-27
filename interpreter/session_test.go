package interpreter

import (
	"testing"
)

// evalString is a test helper that evaluates src and returns its String form,
// or "<nil>" when the fragment produced no value (a statement).
func evalString(t *testing.T, s *Session, src string) string {
	t.Helper()
	v, err := s.Eval(src)
	if err != nil {
		t.Fatalf("Eval(%q) error: %v", src, err)
	}
	if v == nil {
		return "<nil>"
	}
	return v.String()
}

func TestSessionBareExpression(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"1 + 2", "3"},
		{"3 * (4 - 1)", "9"},
		{`"a" + "b"`, "ab"},
		{"true && false", "false"},
		{"!false", "true"},
		{"-5", "-5"},
		{"[1, 2, 3]", "[1, 2, 3]"},
		{"1 < 2", "true"},
	}
	for _, c := range cases {
		s := NewSession(".")
		if got := evalString(t, s, c.src); got != c.want {
			t.Errorf("Eval(%q) = %q, want %q", c.src, got, c.want)
		}
	}
}

// A bare expression entered at the REPL must not leave any binding behind in
// the session scope. The previous implementation wrapped the fragment as an
// assignment to a throwaway variable, which leaked into the scope.
func TestSessionBareExpressionLeavesNoTrace(t *testing.T) {
	s := NewSession(".")
	if _, err := s.Eval("1 + 2"); err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	for name := range s.global.vars {
		t.Errorf("bare expression leaked binding %q into scope", name)
	}
}

// Statements carry state across Eval calls and report no display value.
func TestSessionStatementsPersistAndReturnNil(t *testing.T) {
	s := NewSession(".")
	if v, err := s.Eval("var x = 10"); err != nil || v != nil {
		t.Fatalf("declare: got (%v, %v), want (<nil>, nil)", v, err)
	}
	if got := evalString(t, s, "x + 5"); got != "15" {
		t.Errorf("x + 5 = %q, want 15", got)
	}
	// A reference to the variable alone is also a bare expression.
	if got := evalString(t, s, "x"); got != "10" {
		t.Errorf("x = %q, want 10", got)
	}
}

// A call that parses as a statement (identifier-led) goes through the normal
// path and still yields its return value. print() returns nil, which the REPL
// suppresses from display.
func TestSessionCallStatement(t *testing.T) {
	s := NewSession(".")
	v, err := s.Eval(`print("hi")`)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	if v != nil {
		t.Errorf("print() returned %v, want nil", v)
	}
}

// Genuinely invalid input must surface an error, not be masked by the
// expression-retry path.
func TestSessionInvalidInput(t *testing.T) {
	s := NewSession(".")
	if _, err := s.Eval("var = "); err == nil {
		t.Errorf("expected error for invalid input, got nil")
	}
}
