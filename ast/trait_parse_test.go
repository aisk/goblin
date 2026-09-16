package ast_test

import (
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/source"
)

func TestParseTraitsAndImpls(t *testing.T) {
	src := `trait Shape: Show, geo.Named {
    func area(self)
    func describe(self) {
        return Shape.area(self)
    }
}

type Point(x, y) {
    impl Eq {}
    func text(self) { return 1 }
    impl geo.Named {
        func name(self) { return "p" }
    }
}
`
	mod, err := source.Parse(source.NewLexer([]byte(src)))
	if err != nil {
		t.Fatal(err)
	}
	tr := mod.Body[0].(*ast.TraitDefine)
	if tr.Name != "Shape" || len(tr.Deps) != 2 || tr.Deps[1].String() != "geo.Named" || len(tr.Methods) != 2 {
		t.Fatalf("unexpected trait %+v", tr)
	}
	if tr.Methods[0].Body != nil || tr.Methods[1].Body == nil {
		t.Fatalf("required/default bodies wrong")
	}
	ty := mod.Body[1].(*ast.TypeDefine)
	if len(ty.Methods) != 1 || len(ty.Impls) != 2 || ty.Impls[1].Trait.Module != "geo" || len(ty.Impls[1].Methods) != 1 {
		t.Fatalf("unexpected type %+v", ty)
	}
	if len(ty.AllMethods()) != 2 {
		t.Fatalf("AllMethods = %d", len(ty.AllMethods()))
	}
	if _, err := source.Parse(source.NewLexer([]byte("func f(x)\n"))); err == nil {
		t.Fatal("body-less func outside a trait must not parse")
	}
}
