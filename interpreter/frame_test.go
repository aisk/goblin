package interpreter

import (
	"testing"

	"github.com/aisk/goblin/object"
)

// runFrameProgram runs a checked module and returns the value bound to
// `result`.
func runFrameProgram(t *testing.T, source string) object.Object {
	t.Helper()
	g := runScopeProgram(t, source)
	v, ok := g.Get("result")
	if !ok {
		t.Fatal("program did not bind result")
	}
	return v
}

func expectString(t *testing.T, got object.Object, want string) {
	t.Helper()
	s, err := got.ToString()
	if err != nil {
		t.Fatal(err)
	}
	if s != want {
		t.Errorf("result = %s, want %s", s, want)
	}
}

// Closures created in a loop capture a fresh binding per iteration, both the
// range variable and block declarations, as they did when every block had its
// own scope.
func TestClosuresCapturePerIterationBindings(t *testing.T) {
	expectString(t, runFrameProgram(t, `
var fns = []
for i in [1, 2, 3] {
	var doubled = i * 2
	fns.push(func() { return i * 100 + doubled })
}
var n = 0
while n < 2 {
	var m = n
	fns.push(func() { return m })
	n = n + 1
}
var result = fns.map(func(f) { return f() })
`), "[102, 204, 306, 0, 1]")
}

// A closure shares the variable it captures with the defining function, so
// later writes on either side are visible to the other.
func TestClosuresShareCapturedVariables(t *testing.T) {
	expectString(t, runFrameProgram(t, `
func counter() {
	var count = 0
	var bump = func() {
		count = count + 1
		return count
	}
	bump()
	bump()
	count = count + 10
	return [bump(), count]
}
var result = counter()
`), "[13, 13]")
}

// Variables several functions up resolve through the frame chain.
func TestDeeplyNestedCapture(t *testing.T) {
	expectString(t, runFrameProgram(t, `
func outer(a) {
	if true {
		var b = a + 1
		func middle(c) {
			return func(d) {
				b = b + 1
				return [a, b, c, d]
			}
		}
		return middle(3)
	}
}
var inner = outer(1)
inner(4)
var result = inner(5)
`), "[1, 4, 3, 5]")
}

// Default values run in the defining frame while that frame may be in the
// middle of evaluating another call's arguments; they must not disturb them.
func TestDefaultsDoNotClobberPendingArguments(t *testing.T) {
	expectString(t, runFrameProgram(t, `
func id(x) { return x }
func pick(a = id(100)) { return a }
type Box(v = id(7)) {}
func three(a, b, c) { return [a, b, c] }
var result = three(1, pick(), three(2, Box().v, pick(5)))
`), "[1, 100, [2, 7, 5]]")
}

// Recursion re-enters the same call sites; each activation's arguments stay
// its own.
func TestRecursionKeepsArgumentsApart(t *testing.T) {
	expectString(t, runFrameProgram(t, `
func build(n) {
	if n == 0 {
		return []
	}
	var rest = build(n - 1)
	rest.push([n, build(n - 1).size])
	return rest
}
var result = build(3)
`), "[[1, 0], [2, 1], [3, 2]]")
}

// Return, break and continue travel through nested blocks, loops and try.
func TestControlFlowThroughNestedBlocks(t *testing.T) {
	expectString(t, runFrameProgram(t, `
func find(rows, target) {
	for row in rows {
		var i = 0
		while true {
			i = i + 1
			if i > row.size {
				break
			}
			if row[i - 1] < 0 {
				continue
			}
			try {
				if row[i - 1] == target {
					return [row, i - 1]
				}
			} catch e {
				return e
			}
		}
	}
	return nil
}
var result = [find([[1, -2], [3, 4]], 4), find([[1]], 9)]
`), "[[[3, 4], 1], nil]")
}

// In the unchecked REPL, a function may refer to a name a later line
// defines, and an undefined name is reported when it is reached.
func TestSessionForwardReferences(t *testing.T) {
	s := NewSession(".")
	if _, err := s.Eval("func f() { return later * 2 }"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval("f()"); err == nil {
		t.Fatal("f() before later is defined: want an error")
	}
	if _, err := s.Eval("var later = 21"); err != nil {
		t.Fatal(err)
	}
	if got := evalString(t, s, "f()"); got != "42" {
		t.Errorf("f() = %s, want 42", got)
	}
}
