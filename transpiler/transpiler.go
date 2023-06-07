package transpiler

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/aisk/goblin/ast"
)

var (
	ErrNotImplemented = errors.New("not implemented")
)

func Transpile(mod *ast.Module, output io.Writer) error {
	return transpileModule(mod, output)
}

func transpileModule(mod *ast.Module, output io.Writer) error {
	fmt.Fprintf(output, `
package main

import (
	"github.com/aisk/goblin/builtin"
	"github.com/aisk/goblin/object"
)

func main() {
`)

	if err := transpileStatements(mod.Body, output); err != nil {
		return err
	}

	fmt.Fprintf(output, "}")

	return nil
}

func transpileStatements(stmts []ast.Statement, output io.Writer) error {
	for _, stmt := range stmts {
		switch v := stmt.(type) {
		case ast.FunctionCall:
			if err := transpileFunctionCall(&v, output); err != nil {
				return err
			}
		case ast.Declare:
			if err := transpileDeclare(&v, output); err != nil {
				return err
			}
		case ast.If:
			if err := transpileIf(&v, output); err != nil {
				return err
			}
		case ast.While:
			if err := transpileWhile(&v, output); err != nil {
				return err
			}
		case ast.FunctionDefine:
			if err := transpileFunctionDefine(&v, output); err != nil {
				return err
			}
		case ast.Return:
			if err := transpileReturn(&v, output); err != nil {
				return err
			}
		default:
			return ErrNotImplemented
		}
	}

	return nil
}

func resolveFunctionName(name string) string {
	switch name {
	case "print":
		return "builtin.Print"
	}
	return name
}

func transpileFunctionCall(fn *ast.FunctionCall, output io.Writer) error {
	fmt.Fprintf(output, "%s([]object.Object{", resolveFunctionName(fn.Name))
	for i, arg := range fn.Args {
		if err := transpileExpression(arg, output); err != nil {
			return err
		}
		if i < len(fn.Args)-1 {
			fmt.Fprintf(output, ", ")
		}
	}
	fmt.Fprintf(output, "}, nil)\n") // TODO: add kwargs support

	return nil
}

func transpileFunctionCallToString(fn *ast.FunctionCall) (string, error) {
	buf := bytes.Buffer{}
	if err := transpileFunctionCall(fn, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func transpileFunctionDefine(fn *ast.FunctionDefine, output io.Writer) error {
	fmt.Fprintf(output, "%s := func(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {", fn.Name)

	for i, arg := range fn.Args {
		fmt.Fprintf(output, "var %s = args[%d]; _ = %s\n", arg, i, arg)
	}

	if err := transpileStatements(fn.Body, output); err != nil {
		return err
	}
	fmt.Fprintf(output, "}; _ = %s\n", fn.Name)

	return nil
}

func transpileDeclare(decl *ast.Declare, output io.Writer) error {
	e, err := transpileExpressionToString(decl.Value)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, `var %s = %s; _ = %s`, decl.Name, e, decl.Name)
	fmt.Fprint(output, "\n")
	return nil
}

func transpileIf(if_ *ast.If, output io.Writer) error {
	fmt.Fprint(output, "if ")
	if err := transpileExpression(if_.Condition, output); err != nil {
		return err
	}
	fmt.Fprint(output, ".Bool() {\n")

	if err := transpileStatements(if_.Body, output); err != nil {
		return err
	}

	fmt.Fprintf(output, "}\n")

	return nil
}

func transpileWhile(while_ *ast.While, output io.Writer) error {
	fmt.Fprint(output, "for ")
	if err := transpileExpression(while_.Condition, output); err != nil {
		return err
	}
	fmt.Fprint(output, ".Bool() {\n")

	if err := transpileStatements(while_.Body, output); err != nil {
		return err
	}

	fmt.Fprintf(output, "}\n")

	return nil
}

func transpileExpression(expr ast.Expression, output io.Writer) error {
	s, err := transpileExpressionToString(expr)
	if err != nil {
		return err
	}
	fmt.Fprint(output, s)
	return nil
}

func transpileExpressionToString(expr ast.Expression) (string, error) {
	switch e := expr.(type) {
	case ast.Literal:
		return e.Value.Repr(), nil
	case ast.Symbol:
		return e.Name, nil
	case ast.FunctionCall:
		f, err := transpileFunctionCallToString(&e)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`func() object.Object {
			r, _ := %s // XXX: handle the error
			return r
		} ()`, f), nil
	default:
		return "", ErrNotImplemented
	}
}

func transpileReturn(ret *ast.Return, output io.Writer) error {
	fmt.Fprintf(output, "return ")
	if err := transpileExpression(ret.Value, output); err != nil {
		return err
	}
	fmt.Fprintf(output, ", nil\n")
	return nil
}
