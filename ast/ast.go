package ast

import (
	"fmt"
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
	var args []Expression = nil
	if y != nil {
		args = y.([]Expression)
	}
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

type Identifier struct {
	expressionMixin
	Name string
}

func NewIdentifier(x any) (any, error) {
	s := string(x.(*token.Token).Lit)
	return &Identifier{Name: s}, nil
}

type Assign struct {
	statementMixin
	Target string
	Value  Expression
}

func NewAssign(x, y any) (any, error) {
	name := string(x.(*token.Token).Lit)
	value := y.(Expression)
	return &Assign{
		Target: name,
		Value:  value,
	}, nil
}

type IfElse struct {
	statementMixin
	Condition Expression
	IfBody    []Statement
	ElseBody  []Statement
}

func NewIf(x, y, z any) (any, error) {
	condition := x.(Expression)
	ifBody := y.([]Statement)
	var elseBody []Statement = nil
	if ifElse, ok := z.(*IfElse); ok {
		elseBody = []Statement{ifElse}
	} else if z != nil {
		elseBody = z.([]Statement)
	}
	return &IfElse{
		Condition: condition,
		IfBody:    ifBody,
		ElseBody:  elseBody,
	}, nil
}

type While struct {
	statementMixin
	Condition Expression
	Body      []Statement
}

func NewWhile(x, y any) (any, error) {
	condition := x.(Expression)
	body := y.([]Statement)
	return &While{
		Condition: condition,
		Body:      body,
	}, nil
}

type Break struct {
	statementMixin
}

func NewBreak() (any, error) {
	return &Break{}, nil
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
	s = s[1 : len(s)-1]
	return &Literal{Value: object.String(s)}, nil
}

func NewTrueLiteral() (any, error) {
	return &Literal{Value: object.True}, nil
}

func NewFalseLiteral() (any, error) {
	return &Literal{Value: object.False}, nil
}

func NewNilLiteral() (any, error) {
	return &Literal{Value: object.Nil}, nil
}

type ListLiteral struct {
	expressionMixin
	Elements []Expression
}

func NewListLiteral(x any) (any, error) {
	var elements []Expression
	if x != nil {
		elements = x.([]Expression)
	}
	return &ListLiteral{
		Elements: elements,
	}, nil
}

type FunctionDefine struct {
	statementMixin
	Name       string
	Parameters []string
	Body       []Statement
}

func NewFunctionDefine(x, y any) (any, error) {
	name := string(x.(*token.Token).Lit)
	var body []Statement
	if y != nil {
		body = y.([]Statement)
	}
	// Always insert a return block at the end of function define.
	body = append(body, &Return{Value: &Literal{Value: object.Nil}})
	return &FunctionDefine{
		Name:       name,
		Parameters: []string{},
		Body:       body,
	}, nil
}

type Return struct {
	statementMixin
	Value Expression
}

func NewReturn(x any) (any, error) {
	return &Return{
		Value: x.(Expression),
	}, nil
}

var (
	Add      = "+"
	Minus    = "-"
	Multiply = "*"
	Divide   = "/"
	And      = "&&"
	Or       = "||"
	Not      = "!"
)

type BinaryOperation struct {
	expressionMixin
	LHS      Expression
	RHS      Expression
	Operator string
}

func NewBinaryOperation(lhs, operator, rhs any) (any, error) {
	switch operator.(string) {
	case Add, Minus, Multiply, Divide, And, Or:
	default:
		return nil, fmt.Errorf("invalid operator: '%s'", operator)
	}
	return &BinaryOperation{
		LHS:      lhs.(Expression),
		RHS:      rhs.(Expression),
		Operator: operator.(string),
	}, nil
}

type UnaryOperation struct {
	expressionMixin
	Operand  Expression
	Operator string
}

func NewUnaryOperation(operator, operand any) (any, error) {
	switch operator.(string) {
	case Not:
	default:
		return nil, fmt.Errorf("invalid unary operator: '%s'", operator)
	}
	return &UnaryOperation{
		Operand:  operand.(Expression),
		Operator: operator.(string),
	}, nil
}
