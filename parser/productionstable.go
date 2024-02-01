// Code generated by gocc; DO NOT EDIT.

package parser

import (
    "github.com/aisk/goblin/ast"
)

type (
	ProdTab      [numProductions]ProdTabEntry
	ProdTabEntry struct {
		String     string
		Id         string
		NTType     int
		Index      int
		NumSymbols int
		ReduceFunc func([]Attrib, interface{}) (Attrib, error)
	}
	Attrib interface {
	}
)

var productionsTable = ProdTab{
	ProdTabEntry{
		String: `S' : Module	<<  >>`,
		Id:         "S'",
		NTType:     0,
		Index:      0,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `Module : Statements	<< ast.NewModule(X[0]) >>`,
		Id:         "Module",
		NTType:     1,
		Index:      1,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.NewModule(X[0])
		},
	},
	ProdTabEntry{
		String: `Statements : empty	<<  >>`,
		Id:         "Statements",
		NTType:     2,
		Index:      2,
		NumSymbols: 0,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return nil, nil
		},
	},
	ProdTabEntry{
		String: `Statements : StatementList	<<  >>`,
		Id:         "Statements",
		NTType:     2,
		Index:      3,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `StatementList : Statement	<< ast.NewStatementList(X[0]) >>`,
		Id:         "StatementList",
		NTType:     3,
		Index:      4,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.NewStatementList(X[0])
		},
	},
	ProdTabEntry{
		String: `StatementList : StatementList Statement	<< ast.AppendStatementList(X[0], X[1]) >>`,
		Id:         "StatementList",
		NTType:     3,
		Index:      5,
		NumSymbols: 2,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.AppendStatementList(X[0], X[1])
		},
	},
	ProdTabEntry{
		String: `Statement : Expression	<< X[0], nil >>`,
		Id:         "Statement",
		NTType:     4,
		Index:      6,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `Statement : Declare	<< X[0], nil >>`,
		Id:         "Statement",
		NTType:     4,
		Index:      7,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `Statement : Assign	<< X[0], nil >>`,
		Id:         "Statement",
		NTType:     4,
		Index:      8,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `Statement : If	<<  >>`,
		Id:         "Statement",
		NTType:     4,
		Index:      9,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `Statement : IfElse	<<  >>`,
		Id:         "Statement",
		NTType:     4,
		Index:      10,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `Statement : While	<<  >>`,
		Id:         "Statement",
		NTType:     4,
		Index:      11,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `Expressions : empty	<<  >>`,
		Id:         "Expressions",
		NTType:     5,
		Index:      12,
		NumSymbols: 0,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return nil, nil
		},
	},
	ProdTabEntry{
		String: `Expressions : ExpressionList	<<  >>`,
		Id:         "Expressions",
		NTType:     5,
		Index:      13,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `ExpressionList : Expression	<< ast.NewExpressionList(X[0]) >>`,
		Id:         "ExpressionList",
		NTType:     6,
		Index:      14,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.NewExpressionList(X[0])
		},
	},
	ProdTabEntry{
		String: `ExpressionList : ExpressionList Expression	<< ast.AppendStatementList(X[0], X[1]) >>`,
		Id:         "ExpressionList",
		NTType:     6,
		Index:      15,
		NumSymbols: 2,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.AppendStatementList(X[0], X[1])
		},
	},
	ProdTabEntry{
		String: `Expression : IntegerLiteral	<< X[0], nil >>`,
		Id:         "Expression",
		NTType:     7,
		Index:      16,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `Expression : StringLiteral	<< X[0], nil >>`,
		Id:         "Expression",
		NTType:     7,
		Index:      17,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `Expression : FunctionCall	<< X[0], nil >>`,
		Id:         "Expression",
		NTType:     7,
		Index:      18,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `IntegerLiteral : int_lit	<< ast.NewIntegerLiteral(X[0]) >>`,
		Id:         "IntegerLiteral",
		NTType:     8,
		Index:      19,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.NewIntegerLiteral(X[0])
		},
	},
	ProdTabEntry{
		String: `StringLiteral : string_lit	<< ast.NewStringLiteral(X[0]) >>`,
		Id:         "StringLiteral",
		NTType:     9,
		Index:      20,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.NewStringLiteral(X[0])
		},
	},
	ProdTabEntry{
		String: `FunctionCall : id "(" Arguments ")"	<< ast.NewFunctionCall(X[0], X[2]) >>`,
		Id:         "FunctionCall",
		NTType:     10,
		Index:      21,
		NumSymbols: 4,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.NewFunctionCall(X[0], X[2])
		},
	},
	ProdTabEntry{
		String: `Arguments : empty	<<  >>`,
		Id:         "Arguments",
		NTType:     11,
		Index:      22,
		NumSymbols: 0,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return nil, nil
		},
	},
	ProdTabEntry{
		String: `Arguments : ArgumentList	<<  >>`,
		Id:         "Arguments",
		NTType:     11,
		Index:      23,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `ArgumentList : Expression	<< ast.NewExpressionList(X[0]) >>`,
		Id:         "ArgumentList",
		NTType:     12,
		Index:      24,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.NewExpressionList(X[0])
		},
	},
	ProdTabEntry{
		String: `ArgumentList : ArgumentList "," Expression	<< ast.AppendExpressionList(X[0], X[2]) >>`,
		Id:         "ArgumentList",
		NTType:     12,
		Index:      25,
		NumSymbols: 3,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.AppendExpressionList(X[0], X[2])
		},
	},
	ProdTabEntry{
		String: `Declare : "var" id "=" Expression	<< ast.NewDeclare(X[1], X[3]) >>`,
		Id:         "Declare",
		NTType:     13,
		Index:      26,
		NumSymbols: 4,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.NewDeclare(X[1], X[3])
		},
	},
	ProdTabEntry{
		String: `Assign : id "=" Expression	<< ast.NewAssign(X[0], X[2]) >>`,
		Id:         "Assign",
		NTType:     14,
		Index:      27,
		NumSymbols: 3,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.NewAssign(X[0], X[2])
		},
	},
	ProdTabEntry{
		String: `Block : "{" Statements "}"	<< X[1], nil >>`,
		Id:         "Block",
		NTType:     15,
		Index:      28,
		NumSymbols: 3,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[1], nil
		},
	},
	ProdTabEntry{
		String: `Condition : Expression	<< X[0], nil >>`,
		Id:         "Condition",
		NTType:     16,
		Index:      29,
		NumSymbols: 1,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return X[0], nil
		},
	},
	ProdTabEntry{
		String: `If : "if" Condition Block	<< ast.NewIf(X[1], X[2], nil) >>`,
		Id:         "If",
		NTType:     17,
		Index:      30,
		NumSymbols: 3,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.NewIf(X[1], X[2], nil)
		},
	},
	ProdTabEntry{
		String: `IfElse : "if" Condition Block "else" Block	<< ast.NewIf(X[1], X[2], X[4]) >>`,
		Id:         "IfElse",
		NTType:     18,
		Index:      31,
		NumSymbols: 5,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.NewIf(X[1], X[2], X[4])
		},
	},
	ProdTabEntry{
		String: `While : "while" Condition Block	<< ast.NewWhile(X[1], X[2]) >>`,
		Id:         "While",
		NTType:     19,
		Index:      32,
		NumSymbols: 3,
		ReduceFunc: func(X []Attrib, C interface{}) (Attrib, error) {
			return ast.NewWhile(X[1], X[2])
		},
	},
}
