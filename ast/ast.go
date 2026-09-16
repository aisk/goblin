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

// statements converts a reduced Statements node to a slice. An empty block
// (and an empty module) reduces through `Statements : empty`, which yields a
// nil any rather than an empty slice, so every constructor taking a block
// must go through here instead of asserting the type directly.
func statements(x any) []Statement {
	if x == nil {
		return nil
	}
	return x.([]Statement)
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

type CallArgument struct {
	Expr      Expression
	Name      string
	Kind      CallArgumentKind
	NamePos   token.Pos
	MarkerPos token.Pos
}

type CallArgumentKind int

const (
	CallArgumentPositional CallArgumentKind = iota
	CallArgumentStarred
	CallArgumentKeyword
	CallArgumentKeywordUnpack
)

func NewPositionalArgument(x any) (any, error) {
	return &CallArgument{
		Expr: x.(Expression),
		Kind: CallArgumentPositional,
	}, nil
}

func NewStarredArgument(x any) (any, error) {
	return &CallArgument{
		Expr:      x.(Expression),
		Kind:      CallArgumentStarred,
		MarkerPos: PositionOf(x),
	}, nil
}

func NewSpreadArgument(x any) (any, error) {
	return NewStarredArgument(x)
}

func NewKeywordArgument(name, expr any) (any, error) {
	tok := name.(*token.Token)
	return &CallArgument{
		Expr:    expr.(Expression),
		Name:    string(tok.Lit),
		NamePos: tok.Pos,
		Kind:    CallArgumentKeyword,
	}, nil
}

func NewKeywordUnpackArgument(x any) (any, error) {
	return &CallArgument{
		Expr:      x.(Expression),
		Kind:      CallArgumentKeywordUnpack,
		MarkerPos: PositionOf(x),
	}, nil
}

func NewKeywordSpreadArgument(x any) (any, error) {
	return NewKeywordUnpackArgument(x)
}

func NewCallArgumentList(x any) (any, error) {
	return []CallArgument{*x.(*CallArgument)}, nil
}

func AppendCallArgumentList(l any, x any) (any, error) {
	return append(l.([]CallArgument), *x.(*CallArgument)), nil
}

type FunctionCall struct {
	expressionMixin
	Name string
	Args []CallArgument
}

func NewFunctionCall(x, y any) (any, error) {
	tok := x.(*token.Token)
	name := string(tok.Lit)
	var args []CallArgument
	if y != nil {
		args = y.([]CallArgument)
	}
	return &FunctionCall{
		expressionMixin: expressionMixin{statementMixin{Pos: tok.Pos}},
		Name:            name,
		Args:            args,
	}, nil
}

type CallExpression struct {
	expressionMixin
	Callee Expression
	Args   []CallArgument
}

func NewCallExpression(callee, args any) (any, error) {
	var argList []CallArgument
	if args != nil {
		argList = args.([]CallArgument)
	}
	return &CallExpression{
		expressionMixin: expressionMixin{statementMixin{Pos: PositionOf(callee)}},
		Callee:          callee.(Expression),
		Args:            argList,
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

// SetIndex assigns to an index target, e.g. `list[0] = x` or `dict["k"] = v`.
type SetIndex struct {
	statementMixin
	Object Expression
	Index  Expression
	Value  Expression
}

// SetAttr assigns to a member target, e.g. `obj.field = x`.
type SetAttr struct {
	statementMixin
	Object   Expression
	Property string
	Value    Expression
}

// NewSetAssign builds a SetIndex or SetAttr from an assignment whose target is
// an index or member expression. Other expression targets are rejected.
func NewSetAssign(target, value any) (any, error) {
	v := value.(Expression)
	switch t := target.(type) {
	case *Identifier:
		return &Assign{
			statementMixin: statementMixin{Pos: t.Position()},
			Target:         t.Name,
			Value:          v,
		}, nil
	case *IndexExpression:
		return &SetIndex{
			statementMixin: statementMixin{Pos: t.Position()},
			Object:         t.Object,
			Index:          t.Index,
			Value:          v,
		}, nil
	case *MemberExpression:
		return &SetAttr{
			statementMixin: statementMixin{Pos: t.Position()},
			Object:         t.Object,
			Property:       t.Property,
			Value:          v,
		}, nil
	default:
		return nil, fmt.Errorf("cannot assign to this expression")
	}
}

type IfElse struct {
	statementMixin
	Condition Expression
	IfBody    []Statement
	ElseBody  []Statement
}

func NewIf(x, y, z any) (any, error) {
	condition := x.(Expression)
	ifBody := statements(y)
	var elseBody []Statement = nil
	if ifElse, ok := z.(*IfElse); ok {
		elseBody = []Statement{ifElse}
	} else if z != nil {
		elseBody = statements(z)
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
	body := statements(y)
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
	body := statements(z)
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

type Continue struct {
	statementMixin
}

func NewContinue() (any, error) {
	return &Continue{}, nil
}

type Module struct {
	Name string
	Body []Statement
}

func NewModule(x any) (any, error) {
	return &Module{
		Name: "main",
		Body: statements(x),
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
	s = unescapeString(s[1 : len(s)-1])
	return &Literal{
		expressionMixin: expressionMixin{statementMixin{Pos: tok.Pos}},
		Value:           object.String(s),
	}, nil
}

// unescapeString interprets backslash escape sequences in a string literal's
// raw body (quotes already stripped). The lexer only accepts \n, \t, \r, \"
// and \\ (anything else is a lex error) but keeps the sequences verbatim, so
// we resolve them here.
func unescapeString(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
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
	Parameters []*Parameter
	Body       []Statement
}

type TypeField struct {
	Name         string
	Pos          token.Pos
	DefaultValue Expression
}

func (f *TypeField) HasDefault() bool {
	return f.DefaultValue != nil
}

type Parameter struct {
	Name    string
	Pos     token.Pos
	VarArgs bool
	KwArgs  bool
	Default Expression
}

func (p *Parameter) HasDefault() bool {
	return p.Default != nil
}

func NewRequiredParameter(x any) (any, error) {
	tok := x.(*token.Token)
	return &Parameter{
		Name:    string(tok.Lit),
		Pos:     tok.Pos,
		VarArgs: false,
		KwArgs:  false,
	}, nil
}

func NewDefaultParameter(name, value any) (any, error) {
	tok := name.(*token.Token)
	return &Parameter{
		Name:    string(tok.Lit),
		Pos:     tok.Pos,
		Default: value.(Expression),
	}, nil
}

func NewVarArgsParameter(x any) (any, error) {
	tok := x.(*token.Token)
	return &Parameter{
		Name:    string(tok.Lit),
		Pos:     tok.Pos,
		VarArgs: true,
		KwArgs:  false,
	}, nil
}

func NewVariadicParameter(x any) (any, error) {
	return NewVarArgsParameter(x)
}

func NewKwArgsParameter(x any) (any, error) {
	tok := x.(*token.Token)
	return &Parameter{
		Name:    string(tok.Lit),
		Pos:     tok.Pos,
		VarArgs: false,
		KwArgs:  true,
	}, nil
}

func NewKwVariadicParameter(x any) (any, error) {
	return NewKwArgsParameter(x)
}

func NewParameterList(x any) (any, error) {
	return []*Parameter{x.(*Parameter)}, nil
}

func AppendParameterList(l any, x any) (any, error) {
	return append(l.([]*Parameter), x.(*Parameter)), nil
}

func NewRequiredTypeField(x any) (any, error) {
	tok := x.(*token.Token)
	return &TypeField{
		Name: string(tok.Lit),
		Pos:  tok.Pos,
	}, nil
}

func NewDefaultTypeField(name, value any) (any, error) {
	tok := name.(*token.Token)
	return &TypeField{
		Name:         string(tok.Lit),
		Pos:          tok.Pos,
		DefaultValue: value.(Expression),
	}, nil
}

func NewTypeFieldList(x any) (any, error) {
	return []*TypeField{x.(*TypeField)}, nil
}

func AppendTypeFieldList(l any, x any) (any, error) {
	return append(l.([]*TypeField), x.(*TypeField)), nil
}

func NewFunctionDefine(x, params, y any) (any, error) {
	tok := x.(*token.Token)
	name := string(tok.Lit)
	var parameters []*Parameter
	if params != nil {
		parameters = params.([]*Parameter)
	}
	body := statements(y)
	// Always insert a return block at the end of function define.
	body = append(body, &Return{Value: &Literal{Value: object.Nil}})
	return &FunctionDefine{
		statementMixin: statementMixin{Pos: tok.Pos},
		Name:           name,
		Parameters:     parameters,
		Body:           body,
	}, nil
}

// FunctionLiteral is an anonymous function used as an expression, e.g.
// `var f = func(x) { return x + 1 }`. It captures its defining scope (closure).
type FunctionLiteral struct {
	expressionMixin
	Parameters []*Parameter
	Body       []Statement
}

func NewFunctionLiteral(fn, params, y any) (any, error) {
	tok := fn.(*token.Token)
	var parameters []*Parameter
	if params != nil {
		parameters = params.([]*Parameter)
	}
	body := statements(y)
	// Always insert a return block at the end, mirroring NewFunctionDefine.
	body = append(body, &Return{Value: &Literal{Value: object.Nil}})
	return &FunctionLiteral{
		expressionMixin: expressionMixin{statementMixin{Pos: tok.Pos}},
		Parameters:      parameters,
		Body:            body,
	}, nil
}

type TypeDefine struct {
	statementMixin
	Name    string
	Fields  []*TypeField
	Methods []*FunctionDefine
	// Impls lists the type's impl blocks in declaration order.
	Impls []*ImplBlock
}

// AllMethods lists every method body the type declares: its ordinary methods
// followed by the methods of each impl block. Passes that only care about
// function bodies (name collection, inference) walk this instead of the two
// lists separately.
func (t *TypeDefine) AllMethods() []*FunctionDefine {
	if len(t.Impls) == 0 {
		return t.Methods
	}
	all := append([]*FunctionDefine(nil), t.Methods...)
	for _, impl := range t.Impls {
		all = append(all, impl.Methods...)
	}
	return all
}

func NewTypeMethodList(x any) (any, error) {
	return []*FunctionDefine{x.(*FunctionDefine)}, nil
}

func AppendTypeMethodList(l any, x any) (any, error) {
	return append(l.([]*FunctionDefine), x.(*FunctionDefine)), nil
}

func NewTypeMemberList(x any) (any, error) {
	return []any{x}, nil
}

func AppendTypeMemberList(l any, x any) (any, error) {
	return append(l.([]any), x), nil
}

// TraitRef names a trait in an impl block or a dependency list, either bare
// (`Eq`) or qualified by an imported module (`shapes.Shape`).
type TraitRef struct {
	Module string
	Name   string
	Pos    token.Pos
}

// String spells the reference the way the source did.
func (r *TraitRef) String() string {
	if r.Module == "" {
		return r.Name
	}
	return r.Module + "." + r.Name
}

func NewTraitRef(module, name any) (any, error) {
	tok := name.(*token.Token)
	ref := &TraitRef{Name: string(tok.Lit), Pos: tok.Pos}
	if module != nil {
		modTok := module.(*token.Token)
		ref.Module = string(modTok.Lit)
		ref.Pos = modTok.Pos
	}
	return ref, nil
}

func NewTraitRefList(x any) (any, error) {
	return []*TraitRef{x.(*TraitRef)}, nil
}

func AppendTraitRefList(l any, x any) (any, error) {
	return append(l.([]*TraitRef), x.(*TraitRef)), nil
}

// ImplBlock is one type's implementation of one trait.
type ImplBlock struct {
	Trait   *TraitRef
	Methods []*FunctionDefine
}

func NewImplBlock(trait, methods any) (any, error) {
	impl := &ImplBlock{Trait: trait.(*TraitRef)}
	if methods != nil {
		impl.Methods = methods.([]*FunctionDefine)
	}
	return impl, nil
}

// TraitDefine declares a trait. A method with a nil Body is required; one
// with a body is a default.
type TraitDefine struct {
	statementMixin
	Name    string
	Deps    []*TraitRef
	Methods []*FunctionDefine
}

func NewTraitDefine(name, deps, methods any) (any, error) {
	tok := name.(*token.Token)
	def := &TraitDefine{
		statementMixin: statementMixin{Pos: tok.Pos},
		Name:           string(tok.Lit),
	}
	if deps != nil {
		def.Deps = deps.([]*TraitRef)
	}
	if methods != nil {
		def.Methods = methods.([]*FunctionDefine)
	}
	return def, nil
}

// NewRequiredMethod builds a body-less trait method declaration.
func NewRequiredMethod(name, params any) (any, error) {
	tok := name.(*token.Token)
	var parameters []*Parameter
	if params != nil {
		parameters = params.([]*Parameter)
	}
	return &FunctionDefine{
		statementMixin: statementMixin{Pos: tok.Pos},
		Name:           string(tok.Lit),
		Parameters:     parameters,
	}, nil
}

func NewTypeDefine(name, fields, members any) (any, error) {
	tok := name.(*token.Token)

	var typeFields []*TypeField
	if fields != nil {
		typeFields = fields.([]*TypeField)
	}

	def := &TypeDefine{
		statementMixin: statementMixin{Pos: tok.Pos},
		Name:           string(tok.Lit),
		Fields:         typeFields,
	}
	if members != nil {
		for _, member := range members.([]any) {
			switch m := member.(type) {
			case *FunctionDefine:
				def.Methods = append(def.Methods, m)
			case *ImplBlock:
				def.Impls = append(def.Impls, m)
			}
		}
	}
	return def, nil
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

// NewReturnNil builds a bare `return` (no expression), which returns nil.
// The synthesized nil literal lets the transpiler and interpreter treat it
// like any other return without special-casing a missing value.
func NewReturnNil(x any) (any, error) {
	tok := x.(*token.Token)
	return &Return{
		statementMixin: statementMixin{Pos: tok.Pos},
		Value:          &Literal{Value: object.Nil},
	}, nil
}

type Raise struct {
	statementMixin
	Value Expression
}

func NewRaise(x any) (any, error) {
	return &Raise{
		statementMixin: statementMixin{Pos: PositionOf(x)},
		Value:          x.(Expression),
	}, nil
}

type TryCatch struct {
	statementMixin
	TryBody   []Statement
	CatchVar  string
	CatchBody []Statement
}

func NewTryCatch(tryTok, tryBody, catchVar, catchBody any) (any, error) {
	tok := tryTok.(*token.Token)
	varTok := catchVar.(*token.Token)
	return &TryCatch{
		statementMixin: statementMixin{Pos: tok.Pos},
		TryBody:        statements(tryBody),
		CatchVar:       string(varTok.Lit),
		CatchBody:      statements(catchBody),
	}, nil
}

var (
	Add            = "+"
	Minus          = "-"
	Multiply       = "*"
	Divide         = "/"
	Modulo         = "%"
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
	case Add, Minus, Multiply, Divide, Modulo, And, Or, Equal, NotEqual, LessThan, GreaterThan, LessOrEqual, GreaterOrEqual:
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
	tok := operator.(*token.Token)
	op := string(tok.Lit)
	switch op {
	case Add, Minus, Not:
	default:
		return nil, fmt.Errorf("invalid unary operator: '%s'", op)
	}
	return &UnaryOperation{
		expressionMixin: expressionMixin{statementMixin{Pos: tok.Pos}},
		Operand:         operand.(Expression),
		Operator:        op,
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
