package ast

import "github.com/aisk/goblin/object"

type Statement interface {
	IsStatement()
}

type statementMixin struct{}

func (statementMixin) IsStatement() {}

type Expression interface {
	IsExpression()
}

type expressionMixin struct {
	statementMixin
}

func (expressionMixin) IsExpression() {}

type FunctionCall struct {
	expressionMixin
	Name   string
	Args   []Expression
	KwArgs map[string]Expression
}

type Declare struct {
	expressionMixin
	Name  string
	Value Expression
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

type Literal struct {
	expressionMixin
	Value object.Object
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
