package interpreter

import (
	"reflect"
	"sort"
	"testing"

	"github.com/aisk/goblin/object"
)

func TestSessionArgv(t *testing.T) {
	s := NewSession(".")
	if _, err := s.Eval(`import "os"`); err != nil {
		t.Fatal(err)
	}
	v, err := s.Eval(`os.argv()`)
	if err != nil {
		t.Fatal(err)
	}
	list, ok := v.(*object.List)
	if !ok {
		t.Fatalf("os.argv() = %T, want *object.List", v)
	}
	if len(list.Elements) != 1 {
		t.Fatalf("os.argv() size = %d, want 1", len(list.Elements))
	}
	got, ok := list.Elements[0].(object.String)
	if !ok || string(got) != replArgv0 {
		t.Fatalf("os.argv()[0] = %#v, want %q", list.Elements[0], replArgv0)
	}
}

func TestSessionCompletionCandidates(t *testing.T) {
	s := NewSession(".")
	if names := s.CompletionCandidates(nil); !containsString(names, "print") || !sort.StringsAreSorted(names) {
		t.Fatalf("root completion candidates = %v", names)
	}

	if _, err := s.Eval(`type User(name) { func hello(self) { return self.name } }`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval(`var user = User("alice")`); err != nil {
		t.Fatal(err)
	}
	if names := s.CompletionCandidates([]string{"user"}); !reflect.DeepEqual(names, []string{"name", "hello", "constructor", "attributes"}) {
		t.Fatalf("user completion candidates = %v", names)
	}
	if names := s.CompletionCandidates([]string{"user", "name"}); !containsString(names, "trim") {
		t.Fatalf("nested String completion candidates = %v", names)
	}
	if names := s.CompletionCandidates([]string{"missing"}); names != nil {
		t.Fatalf("missing completion candidates = %v, want nil", names)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

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

func TestSessionUserTypeAttributes(t *testing.T) {
	s := NewSession(".")
	if _, err := s.Eval("type User(name) { func hello(self) { return self.name } }"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval(`var user = User("alice")`); err != nil {
		t.Fatal(err)
	}
	if got := evalString(t, s, "user.attributes()"); got != `["name", "hello", "constructor", "attributes"]` {
		t.Fatalf("user.attributes() = %q", got)
	}
}

func TestSessionUserCanOverrideAttributes(t *testing.T) {
	s := NewSession(".")
	if _, err := s.Eval(`type User() { func attributes(self) { return ["custom"] } }`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval("var user = User()"); err != nil {
		t.Fatal(err)
	}
	if got := evalString(t, s, "user.attributes()"); got != `["custom"]` {
		t.Fatalf("overridden user.attributes() = %q", got)
	}
}
