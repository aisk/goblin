package ast

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/token"
)

// The NewXxx constructors and XxxList structs are shims for gocc.

type Statement interface {
	Position() token.Pos
	IsStatement()
}

type statementMixin struct {
	Pos token.Pos
}

func (statementMixin) IsStatement() {}
func (s statementMixin) Position() token.Pos {
	return s.Pos
}

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

type NamedArgument struct {
	Name  string
	Value Expression
}

type CallArgument struct {
	Name  string
	Value Expression
}

func NewPositionalArgument(x any) (any, error) {
	return &CallArgument{
		Name:  "",
		Value: x.(Expression),
	}, nil
}

func NewNamedArgument(name, value any) (any, error) {
	tok := name.(*token.Token)
	return &CallArgument{
		Name:  string(tok.Lit),
		Value: value.(Expression),
	}, nil
}

func NewArgumentList(x any) (any, error) {
	return []*CallArgument{x.(*CallArgument)}, nil
}

func AppendArgumentList(l any, x any) (any, error) {
	return append(l.([]*CallArgument), x.(*CallArgument)), nil
}

func MergeArgumentLists(l, r any) (any, error) {
	left := l.([]*CallArgument)
	right := r.([]*CallArgument)
	merged := make([]*CallArgument, 0, len(left)+len(right))
	merged = append(merged, left...)
	merged = append(merged, right...)
	return merged, nil
}

func splitCallArguments(args any) ([]Expression, []*NamedArgument) {
	if args == nil {
		return nil, nil
	}
	items := args.([]*CallArgument)
	positional := make([]Expression, 0, len(items))
	named := make([]*NamedArgument, 0, len(items))
	for _, arg := range items {
		if arg.Name == "" {
			positional = append(positional, arg.Value)
			continue
		}
		named = append(named, &NamedArgument{Name: arg.Name, Value: arg.Value})
	}
	return positional, named
}

type FunctionCall struct {
	expressionMixin
	Name   string
	Args   []Expression
	KwArgs []*NamedArgument
}

func NewFunctionCall(x, y any) (any, error) {
	tok := x.(*token.Token)
	name := string(tok.Lit)
	args, kwargs := splitCallArguments(y)
	return &FunctionCall{
		expressionMixin: expressionMixin{statementMixin{Pos: tok.Pos}},
		Name:            name,
		Args:            args,
		KwArgs:          kwargs,
	}, nil
}

type CallExpression struct {
	expressionMixin
	Callee Expression
	Args   []Expression
	KwArgs []*NamedArgument
}

func NewCallExpression(callee, args any) (any, error) {
	argList, kwargList := splitCallArguments(args)
	return &CallExpression{
		expressionMixin: expressionMixin{statementMixin{Pos: PositionOf(callee)}},
		Callee:          callee.(Expression),
		Args:            argList,
		KwArgs:          kwargList,
	}, nil
}

type Declare struct {
	statementMixin
	Name  string
	Value Expression
}

func NewDeclare(x, y any) (any, error) {
	tok := x.(*token.Token)
	name := string(tok.Lit)
	value := y.(Expression)
	return &Declare{
		statementMixin: statementMixin{Pos: tok.Pos},
		Name:           name,
		Value:          value,
	}, nil
}

type Identifier struct {
	expressionMixin
	Name string
}

func NewIdentifier(x any) (any, error) {
	tok := x.(*token.Token)
	s := string(tok.Lit)
	return &Identifier{
		expressionMixin: expressionMixin{statementMixin{Pos: tok.Pos}},
		Name:            s,
	}, nil
}

func NewIndexExpressionFromIdentifier(id, idx any) (any, error) {
	identifier, err := NewIdentifier(id)
	if err != nil {
		return nil, err
	}
	return NewIndexExpression(identifier, idx)
}

func NewCallExpressionFromIdentifier(id, args any) (any, error) {
	identifier, err := NewIdentifier(id)
	if err != nil {
		return nil, err
	}
	return NewCallExpression(identifier, args)
}

func NewMemberExpressionFromIdentifier(id, prop any) (any, error) {
	identifier, err := NewIdentifier(id)
	if err != nil {
		return nil, err
	}
	return NewMemberExpression(identifier, prop)
}

type Assign struct {
	statementMixin
	Target string
	Value  Expression
}

func NewAssign(x, y any) (any, error) {
	tok := x.(*token.Token)
	name := string(tok.Lit)
	value := y.(Expression)
	return &Assign{
		statementMixin: statementMixin{Pos: tok.Pos},
		Target:         name,
		Value:          value,
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
		statementMixin: statementMixin{Pos: PositionOf(x)},
		Condition:      condition,
		IfBody:         ifBody,
		ElseBody:       elseBody,
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
		statementMixin: statementMixin{Pos: PositionOf(x)},
		Condition:      condition,
		Body:           body,
	}, nil
}

type For struct {
	statementMixin
	Variable string
	Iterator Expression
	Body     []Statement
}

func NewFor(x, y, z any) (any, error) {
	tok := x.(*token.Token)
	variable := string(tok.Lit)
	iterator := y.(Expression)
	body := z.([]Statement)
	return &For{
		statementMixin: statementMixin{Pos: tok.Pos},
		Variable:       variable,
		Iterator:       iterator,
		Body:           body,
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
	tok := x.(*token.Token)
	d, err := strconv.Atoi(string(tok.Lit))
	if err != nil {
		return nil, err
	}
	return &Literal{
		expressionMixin: expressionMixin{statementMixin{Pos: tok.Pos}},
		Value:           object.Integer(d),
	}, nil
}

func NewFloatLiteral(x any) (any, error) {
	tok := x.(*token.Token)
	f, err := strconv.ParseFloat(string(tok.Lit), 64)
	if err != nil {
		return nil, err
	}
	return &Literal{
		expressionMixin: expressionMixin{statementMixin{Pos: tok.Pos}},
		Value:           object.Float(f),
	}, nil
}

func NewStringLiteral(x any) (any, error) {
	tok := x.(*token.Token)
	s := string(tok.Lit)
	s = s[1 : len(s)-1]
	return &Literal{
		expressionMixin: expressionMixin{statementMixin{Pos: tok.Pos}},
		Value:           object.String(s),
	}, nil
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
	pos := token.Pos{}
	if len(elements) > 0 {
		pos = elements[0].Position()
	}
	return &ListLiteral{
		expressionMixin: expressionMixin{statementMixin{Pos: pos}},
		Elements:        elements,
	}, nil
}

type DictElement struct {
	Key   Expression
	Value Expression
}

func NewDictElement(key, value any) (any, error) {
	return &DictElement{
		Key:   key.(Expression),
		Value: value.(Expression),
	}, nil
}

func NewDictElementList(x any) (any, error) {
	return []*DictElement{x.(*DictElement)}, nil
}

func AppendDictElementList(l any, x any) (any, error) {
	return append(l.([]*DictElement), x.(*DictElement)), nil
}

type DictLiteral struct {
	expressionMixin
	Elements []*DictElement
}

func NewDictLiteral(x any) (any, error) {
	var elements []*DictElement
	if x != nil {
		elements = x.([]*DictElement)
	}
	pos := token.Pos{}
	if len(elements) > 0 {
		pos = elements[0].Key.Position()
	}
	return &DictLiteral{
		expressionMixin: expressionMixin{statementMixin{Pos: pos}},
		Elements:        elements,
	}, nil
}

type FunctionDefine struct {
	statementMixin
	Name       string
	Parameters []string
	Body       []Statement
}

func NewParameterList(x any) (any, error) {
	name := string(x.(*token.Token).Lit)
	return []string{name}, nil
}

func AppendParameterList(l any, x any) (any, error) {
	name := string(x.(*token.Token).Lit)
	return append(l.([]string), name), nil
}

func NewFunctionDefine(x, params, y any) (any, error) {
	tok := x.(*token.Token)
	name := string(tok.Lit)
	var parameters []string
	if params != nil {
		parameters = params.([]string)
	}
	var body []Statement
	if y != nil {
		body = y.([]Statement)
	}
	// Always insert a return block at the end of function define.
	body = append(body, &Return{Value: &Literal{Value: object.Nil}})
	return &FunctionDefine{
		statementMixin: statementMixin{Pos: tok.Pos},
		Name:           name,
		Parameters:     parameters,
		Body:           body,
	}, nil
}

type Return struct {
	statementMixin
	Value Expression
}

func NewReturn(x any) (any, error) {
	return &Return{
		statementMixin: statementMixin{Pos: PositionOf(x)},
		Value:          x.(Expression),
	}, nil
}

var (
	Add            = "+"
	Minus          = "-"
	Multiply       = "*"
	Divide         = "/"
	And            = "&&"
	Or             = "||"
	Not            = "!"
	Equal          = "=="
	NotEqual       = "!="
	LessThan       = "<"
	GreaterThan    = ">"
	LessOrEqual    = "<="
	GreaterOrEqual = ">="
)

type BinaryOperation struct {
	expressionMixin
	LHS      Expression
	RHS      Expression
	Operator string
}

func NewBinaryOperation(lhs, operator, rhs any) (any, error) {
	switch operator.(string) {
	case Add, Minus, Multiply, Divide, And, Or, Equal, NotEqual, LessThan, GreaterThan, LessOrEqual, GreaterOrEqual:
	default:
		return nil, fmt.Errorf("invalid operator: '%s'", operator)
	}
	return &BinaryOperation{
		expressionMixin: expressionMixin{statementMixin{Pos: lhs.(Expression).Position()}},
		LHS:             lhs.(Expression),
		RHS:             rhs.(Expression),
		Operator:        operator.(string),
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
		expressionMixin: expressionMixin{statementMixin{Pos: operand.(Expression).Position()}},
		Operand:         operand.(Expression),
		Operator:        operator.(string),
	}, nil
}

type IndexExpression struct {
	expressionMixin
	Object Expression
	Index  Expression
}

func NewIndexExpression(obj, idx any) (any, error) {
	return &IndexExpression{
		expressionMixin: expressionMixin{statementMixin{Pos: obj.(Expression).Position()}},
		Object:          obj.(Expression),
		Index:           idx.(Expression),
	}, nil
}

type MemberExpression struct {
	expressionMixin
	Object   Expression
	Property string
}

func NewMemberExpression(obj, prop any) (any, error) {
	propTok := prop.(*token.Token)
	return &MemberExpression{
		expressionMixin: expressionMixin{statementMixin{Pos: propTok.Pos}},
		Object:          obj.(Expression),
		Property:        string(propTok.Lit),
	}, nil
}

type Export struct {
	statementMixin
	Name string
}

func NewExport(x any) (any, error) {
	tok := x.(*token.Token)
	name := string(tok.Lit)
	return &Export{
		statementMixin: statementMixin{Pos: tok.Pos},
		Name:           name,
	}, nil
}

type Import struct {
	statementMixin
	Name string // variable name: builtin uses name directly ("os"), path takes last segment ("bar")
	Path string // original path string ("os", "./foo/bar")
}

func NewImport(x any) (any, error) {
	tok := x.(*token.Token)
	raw := string(tok.Lit)
	path := raw[1 : len(raw)-1] // strip quotes
	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]
	return &Import{
		statementMixin: statementMixin{Pos: tok.Pos},
		Name:           name,
		Path:           path,
	}, nil
}

func PositionOf(v any) token.Pos {
	if v == nil {
		return token.Pos{}
	}
	switch n := v.(type) {
	case Statement:
		return n.Position()
	case Expression:
		return n.Position()
	case interface{ Position() token.Pos }:
		return n.Position()
	case *token.Token:
		return n.Pos
	default:
		return token.Pos{}
	}
}
