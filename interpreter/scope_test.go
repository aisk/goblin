package interpreter

import (
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
)

func runScopeProgram(t *testing.T, source string) *Environment {
	t.Helper()
	node, err := parser.NewParser().Parse(lexer.NewLexer([]byte(source)))
	if err != nil {
		t.Fatal(err)
	}
	mod := node.(*ast.Module)
	if err := semantic.CheckModule(mod); err != nil {
		t.Fatal(err)
	}
	env := NewEnvironment(nil)
	if err := evalStatements(mod.Body, env); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestBlockDeclarationsDoNotEscape(t *testing.T) {
	env := runScopeProgram(t, `
var outer = 1
if true { var from_if = 2 }
while true { var from_while = 3 break }
for item in [4] { var from_for = item }
try { var from_try = 5 raise Error("stop") } catch caught { var from_catch = caught }
`)

	for _, name := range []string{"from_if", "from_while", "item", "from_for", "from_try", "caught", "from_catch"} {
		if _, ok := env.Get(name); ok {
			t.Errorf("block-local binding %q escaped into the outer scope", name)
		}
	}
	if got, ok := env.Get("outer"); !ok || got != object.Integer(1) {
		t.Errorf("outer binding = %v, %v; want 1, true", got, ok)
	}
}

func TestAssignmentUpdatesNearestOuterBinding(t *testing.T) {
	env := runScopeProgram(t, `
var value = 0
if true { value = 1 }
while value < 2 { value = value + 1 }
for item in [3] { value = item }
`)

	if got, ok := env.Get("value"); !ok || got != object.Integer(3) {
		t.Errorf("value = %v, %v; want 3, true", got, ok)
	}
}

func TestForBodyCanShadowIterationBinding(t *testing.T) {
	runScopeProgram(t, `for item in [1] { var item = 2 print(item) }`)
}
