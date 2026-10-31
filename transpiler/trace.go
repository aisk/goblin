package transpiler

import (
	"fmt"
	"path/filepath"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/token"
	"github.com/dave/jennifer/jen"
)

// errHandler generates the error-handling code for a given error variable name.
type errHandler func(errVar string) jen.Code

func frameCode(module, function string, pos token.Pos) *jen.Statement {
	file := ""
	if src, ok := pos.Context.(token.Sourcer); ok && src != nil {
		file = src.Source()
	}
	return jen.Qual(pathObject, "Frame").Values(jen.Dict{
		jen.Id("Module"):   jen.Lit(module),
		jen.Id("Function"): jen.Lit(function),
		jen.Id("File"):     jen.Lit(file),
		jen.Id("Line"):     jen.Lit(pos.Line),
		jen.Id("Column"):   jen.Lit(pos.Column),
	})
}

func tracedReturn(zero *jen.Statement, errVar, module, function string, pos token.Pos) jen.Code {
	return jen.Return(zero, jen.Qual(pathObject, "WithFrame").Call(
		jen.Id(errVar), frameCode(module, function, pos),
	))
}

func modulePosition(mod *ast.Module) token.Pos {
	if len(mod.Body) > 0 {
		return mod.Body[0].Position()
	}
	return token.Pos{}
}

// sourceModuleName derives the traceback module name from a position's source
// file via the shared source.ModuleName, so frames from both backends carry
// the same module tag.
func sourceModuleName(pos token.Pos) string {
	if src, ok := pos.Context.(token.Sourcer); ok && src != nil {
		return source.ModuleName(src.Source())
	}
	return ""
}

// lineDirective returns a jen.Code that renders as a Go `//line` directive
// pointing back to the original .goblin source location. Returns nil if the
// position lacks file context (zero-value Pos, synthetic nodes), in which
// case jen will skip it during rendering.
func lineDirective(pos token.Pos) jen.Code {
	src, ok := pos.Context.(token.Sourcer)
	if !ok || src == nil {
		return nil
	}
	path := src.Source()
	if path == "" || pos.Line <= 0 {
		return nil
	}
	// `go build` runs in a temp dir; absolutize so diagnostics resolve.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	col := pos.Column
	if col <= 0 {
		col = 1
	}
	return jen.Comment(fmt.Sprintf("//line %s:%d:%d", path, pos.Line, col))
}
