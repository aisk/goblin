package interpreter

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/aisk/goblin/object"
)

func TestSessionArgv(t *testing.T) {
	s := NewSession(".")
	if _, err := s.Eval(`import "os"`); err != nil {
		t.Fatal(err)
	}
	v, err := s.Eval(`os.args`)
	if err != nil {
		t.Fatal(err)
	}
	list, ok := v.(*object.List)
	if !ok || len(list.Elements) != 0 {
		t.Fatalf("os.args = %v, want []", v)
	}
	v, err = s.Eval(`os.program`)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := v.(object.String); !ok || string(got) != replArgv0 {
		t.Fatalf("os.program = %#v, want %q", v, replArgv0)
	}
}

// os.exit in a fragment runs the deferred calls of the functions on the way
// and of the fragment, and Eval returns the exit request for the REPL to end
// with.
func TestSessionEvalExit(t *testing.T) {
	s := NewSession(".")
	for _, src := range []string{`import "os"`, `var log = []`, "func quit(code) {\n    defer log.push(\"f\")\n    os.exit(code)\n}"} {
		if _, err := s.Eval(src); err != nil {
			t.Fatal(err)
		}
	}
	v, err := s.Eval("defer log.push(\"d\")\ntry {\n    quit(3)\n} catch e {\n    log.push(\"caught\")\n}")
	if code, ok := object.ExitCode(err); !ok || code != 3 || v != nil {
		t.Fatalf("Eval = %v, %v, want exit 3", v, err)
	}
	v, err = s.Eval(`log`)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(v); got != `["f", "d"]` {
		t.Fatalf("log = %s, want [\"f\", \"d\"]", got)
	}
}

func TestSessionCompletionCandidates(t *testing.T) {
	s := NewSession(".")
	if names := s.CompletionCandidates(nil); !containsString(names, "print") || !containsString(names, "eprint") || !sort.StringsAreSorted(names) {
		t.Fatalf("root completion candidates = %v", names)
	}

	if _, err := s.Eval(`type User(name) { func hello(self) { return self.name } }`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval(`var user = User("alice")`); err != nil {
		t.Fatal(err)
	}
	if names := s.CompletionCandidates([]string{"user"}); !reflect.DeepEqual(names, []string{"name", "hello"}) {
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
	return fmt.Sprint(v)
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
	for _, name := range s.global.names() {
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

// Destructuring at the prompt binds session variables like `var` does, and a
// slice of one is a bare expression.
func TestSessionDestructureAndSlice(t *testing.T) {
	s := NewSession(".")
	if v, err := s.Eval("var a, b = [1, 2]"); err != nil || v != nil {
		t.Fatalf("declare: got (%v, %v), want (<nil>, nil)", v, err)
	}
	if _, err := s.Eval("a, b = [b, a]"); err != nil {
		t.Fatalf("swap: %v", err)
	}
	if got := evalString(t, s, "[a, b, 3][1:]"); got != "[1, 3]" {
		t.Errorf("[a, b, 3][1:] = %q, want [1, 3]", got)
	}
	if _, err := s.Eval("for k, v in [[a, b]] { a = k + v }"); err != nil {
		t.Fatalf("loop: %v", err)
	}
	if got := evalString(t, s, "a"); got != "3" {
		t.Errorf("a = %q, want 3", got)
	}
}

// Each fragment is a body of its own, so what it defers runs when it ends,
// after its other statements and also when one of them fails.
func TestSessionDefer(t *testing.T) {
	s := NewSession(".")
	if _, err := s.Eval("var log = []"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval("defer log.push(\"deferred\")\nlog.push(\"body\")"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval("defer log.push(\"after error\")\nraise Error(\"boom\")"); err == nil {
		t.Fatal("raise: got no error")
	}
	if got := evalString(t, s, "log"); got != `["body", "deferred", "after error"]` {
		t.Errorf("log = %s", got)
	}
	_, err := s.Eval("defer Int(\"x\")")
	if err == nil || !strings.Contains(err.Error(), "x") {
		t.Errorf("failing deferred call: got %v", err)
	}
}

// A call that parses as a statement (identifier-led) goes through the normal
// path and still yields its return value. print()/eprint() return the nil
// object, which the REPL suppresses from display.
func TestSessionCallStatement(t *testing.T) {
	s := NewSession(".")
	for _, expr := range []string{`print("hi")`, `eprint("hi")`} {
		discardStdio(t, func() {
			v, err := s.Eval(expr)
			if err != nil {
				t.Fatalf("Eval(%s) error: %v", expr, err)
			}
			if v != object.Nil {
				t.Errorf("%s returned %v, want object.Nil", expr, v)
			}
		})
	}
}

func discardStdio(t *testing.T, fn func()) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("devnull: %v", err)
	}
	os.Stdout, os.Stderr = devNull, devNull
	defer func() {
		os.Stdout, os.Stderr = origOut, origErr
		_ = devNull.Close()
	}()
	fn()
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
	if got := evalString(t, s, "attributes(user)"); got != `["name", "hello"]` {
		t.Fatalf("attributes(user) = %q", got)
	}
}

// A method named like a reflection function is an ordinary member.
func TestSessionUserMethodNamedAttributes(t *testing.T) {
	s := NewSession(".")
	if _, err := s.Eval(`type User() { func attributes(self) { return ["custom"] } }`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval("var user = User()"); err != nil {
		t.Fatal(err)
	}
	if got := evalString(t, s, "user.attributes()"); got != `["custom"]` {
		t.Fatalf("user.attributes() = %q", got)
	}
	if got := evalString(t, s, "attributes(user)"); got != `["attributes"]` {
		t.Fatalf("attributes(user) = %q", got)
	}
}

// The REPL parses interpolated expressions too, against names earlier lines
// declared.
func TestSessionInterpolation(t *testing.T) {
	s := NewSession(".")
	if _, err := s.Eval(`var n = 2`); err != nil {
		t.Fatal(err)
	}
	v, err := s.Eval(`"n={n}, twice={n * 2}"`)
	if err != nil {
		t.Fatal(err)
	}
	if v != object.String("n=2, twice=4") {
		t.Fatalf("value = %v", v)
	}
}

// The REPL does not run the semantic checker, so defining a type reports an
// unknown protocol itself, with the checker's message, as a TypeError.
func TestSessionUnknownProtocol(t *testing.T) {
	s := NewSession(".")
	_, err := s.Eval("type Rect(w) {\n  impl Shape {}\n}")
	want := "impl Shape for Rect: Shape is not a protocol (protocols: Eq, Ord, Hashable, Show, Truth, Add, Sub, Mul, Div, Mod, Neg, Iter, Index); other methods go directly in the type body"
	if err == nil || !errors.Is(err, object.TypeError) || !strings.Contains(err.Error(), want) {
		t.Fatalf("Eval error = %v, want TypeError %q", err, want)
	}
	// A user type named like a protocol does not change what impl means.
	if _, err := s.Eval("type Show(x) {\n  impl Show {}\n}"); err != nil {
		t.Fatal(err)
	}
	if v, err := s.Eval("String(Show(1))"); err != nil || v.(object.String) != "Show(x=1)" {
		t.Fatalf("String(Show(1)) = %v, %v", v, err)
	}
}

// An assignment in the REPL refuses what the checker refuses in a file: a
// name nothing declared, a type and a built-in without var. The refusal is a
// NameError with the checker's wording, raised when the line runs.
func TestSessionAssignmentNeedsADeclaration(t *testing.T) {
	s := NewSession(".")
	for _, src := range []string{"func f() { z = 5 }", "type T(a) {}", "var q = 1"} {
		if _, err := s.Eval(src); err != nil {
			t.Fatal(err)
		}
	}
	refused := map[string]string{
		"y = 1":     "assignment to undefined identifier: y",
		"f()":       "assignment to undefined identifier: z",
		"T = 4":     "cannot assign to type T",
		"print = 1": "cannot assign to built-in 'print' (declare your own with var)",
		"Int = 1":   "cannot assign to built-in 'Int' (declare your own with var)",
		"break":     "break used outside loop",
		"nope":      "undefined identifier: nope",
	}
	for src, want := range refused {
		_, err := s.Eval(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %v, want %q", src, err, want)
			continue
		}
		if src != "break" && !errors.Is(err, object.NameError) {
			t.Errorf("%s: %v is not a NameError", src, err)
		}
	}
	for _, src := range []string{"q = 2", "var T = 5", "T = 6", "var print = 1", "print = 2", "var y = 1", "y = 2"} {
		if _, err := s.Eval(src); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
	if v, err := s.Eval("[q, T, print, y]"); err != nil || fmt.Sprint(v) != "[2, 6, 2, 2]" {
		t.Fatalf("values = %v, %v", v, err)
	}
}
