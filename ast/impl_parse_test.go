package ast_test

import (
	"strings"
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/source"
)

func TestParseImpls(t *testing.T) {
	src := `type Point(x, y) {
    impl Eq {}
    func text(self) { return 1 }
    impl Show {
        func show(self) { return "p" }
    }
}
`
	mod, err := source.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ty := mod.Body[0].(*ast.TypeDefine)
	if len(ty.Methods) != 1 || len(ty.Impls) != 2 || ty.Impls[0].Protocol != "Eq" || ty.Impls[1].Protocol != "Show" || len(ty.Impls[1].Methods) != 1 {
		t.Fatalf("unexpected type %+v", ty)
	}
	if pos := ty.Impls[1].Pos; pos.Line != 4 || pos.Column != 10 {
		t.Fatalf("impl Show position = %d:%d, want 4:10", pos.Line, pos.Column)
	}
	if len(ty.AllMethods()) != 2 {
		t.Fatalf("AllMethods = %d", len(ty.AllMethods()))
	}
}

func TestParseRejectsTraitSyntax(t *testing.T) {
	for _, src := range []string{
		"type T() {\n    impl a.B {}\n}\n",
		"trait T {}\n",
		"func f(x)\n",
	} {
		if _, err := source.ParseString(src); err == nil {
			t.Errorf("%q parsed, want a syntax error", strings.TrimSpace(src))
		}
	}
	// trait is an ordinary identifier now.
	if _, err := source.ParseString("var trait = 1\nprint(trait)\n"); err != nil {
		t.Fatalf("trait as an identifier: %v", err)
	}
}
