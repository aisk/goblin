package ast

import (
	"strconv"

	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/token"
)

// The NewXxx constructors and XxxList structs are shims for gocc.

type Statement interface {
	IsStatement()
}

type statementMixin struct{}

func (statementMixin) IsStatement() {}

type StatementList []Statement

func NewStatementList(x any) (any, error) {
	return []Statement{x.(Statement)}, nil
}

func AppendStatementList(l any, x any) (any, error) {
	return append(l.([]Statement), x.(Statement)), nil
}

type Expression interface {
	Statement
	IsExpression()
}

type expressionMixin struct {
	statementMixin
}

func (expressionMixin) IsExpression() {}

type ExpressionList []Expression

func NewExpressionList(x any) (any, error) {
	return []Expression{x.(Expression)}, nil
}

func AppendExpressionList(l any, x any) (any, error) {
	return append(l.([]Expression), x.(Expression)), nil
}

type FunctionCall struct {
	expressionMixin
	Name   string
	Args   []Expression
	KwArgs map[string]Expression
}

func NewFunctionCall(x, y any) (any, error) {
	name := string(x.(*token.Token).Lit)
	args := y.([]Expression)
	return &FunctionCall{
		Name: name,
		Args: args,
	}, nil
}

type Declare struct {
	statementMixin
	Name  string
	Value Expression
}

func NewDeclare(x, y any) (any, error) {
	name := string(x.(*token.Token).Lit)
	value := y.(Expression)
	return &Declare{
		Name:  name,
		Value: value,
	}, nil
}

type Symbol struct {
	expressionMixin
	Name string
}

type Assign struct {
	statementMixin
	Target string
	Value  Expression
}

type If struct {
	statementMixin
	Condition Expression
	Body      []Statement
}

type While struct {
	statementMixin
	Condition Expression
	Body      []Statement
}

type Module struct {
	Name string
	Body []Statement
}

func NewModule(x any) (any, error) {
	return &Module{
		Name: "main",
		Body: x.([]Statement),
	}, nil
}

type Literal struct {
	expressionMixin
	Value object.Object
}

func NewIntegerLiteral(x any) (any, error) {
	d, err := strconv.Atoi(string(x.(*token.Token).Lit))
	if err != nil {
		return nil, err
	}
	return &Literal{Value: object.Integer(d)}, nil
}

func NewStringLiteral(x any) (any, error) {
	s := string(x.(*token.Token).Lit)
	return &Literal{Value: object.String(s)}, nil
}

type FunctionDefine struct {
	statementMixin
	Name string
	Args []string
	Body []Statement
}

type Return struct {
	statementMixin
	Value Expression
}
