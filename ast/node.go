// Package ast defines the syntax tree both Goblin backends consume.
//
// node.go holds the node types, grammar.go the constructors the generated
// parser calls from its reduce actions, and walk.go the generic traversal.
package ast

import (
	"strings"

	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/token"
)

// Statement is any node that can appear in a statement list. Every
// expression is also a statement (an expression statement).
type Statement interface {
	Position() token.Pos
	IsStatement()
}

// Expression is a node that produces a value.
type Expression interface {
	Statement
	IsExpression()
}

type statementMixin struct {
	Pos token.Pos
}

func (statementMixin) IsStatement()          {}
func (s statementMixin) Position() token.Pos { return s.Pos }

type expressionMixin struct {
	statementMixin
}

func (expressionMixin) IsExpression() {}

func stmtAt(pos token.Pos) statementMixin  { return statementMixin{Pos: pos} }
func exprAt(pos token.Pos) expressionMixin { return expressionMixin{statementMixin{Pos: pos}} }

// Module is a parsed source file.
type Module struct {
	Name string
	Body []Statement
}

// --- statements

// Declare is `var Name = Value`.
type Declare struct {
	statementMixin
	Name  string
	Value Expression
}

// Assign is `Target = Value` for a plain name.
type Assign struct {
	statementMixin
	Target string
	Value  Expression
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

// IfElse is an if statement. An `else if` chain nests: ElseBody then holds
// a single *IfElse.
type IfElse struct {
	statementMixin
	Condition Expression
	IfBody    []Statement
	ElseBody  []Statement
}

type While struct {
	statementMixin
	Condition Expression
	Body      []Statement
}

// For is `for Variable in Iterator { Body }`.
type For struct {
	statementMixin
	Variable string
	Iterator Expression
	Body     []Statement
}

type Break struct {
	statementMixin
}

type Continue struct {
	statementMixin
}

// Return always carries a value: a bare `return` and the implicit return
// appended to every function body return a nil literal.
type Return struct {
	statementMixin
	Value Expression
}

type Raise struct {
	statementMixin
	Value Expression
}

type TryCatch struct {
	statementMixin
	TryBody   []Statement
	CatchVar  string
	CatchBody []Statement
}

// Export marks a module-level name as a member of the module value.
type Export struct {
	statementMixin
	Name string
}

// Import binds a module to Name, the last segment of Path ("os" for "os",
// "bar" for "./foo/bar").
type Import struct {
	statementMixin
	Name string
	Path string
}

// IsPathImport reports whether the import names a .goblin file on disk rather
// than a stdlib module. File imports must be explicitly relative ("./" or
// "../"); every other path, slashes included, is a stdlib module name (e.g.
// "x/compress/gzip").
func (i *Import) IsPathImport() bool {
	return IsPathImport(i.Path)
}

// IsPathImport is Import.IsPathImport for a bare path string.
func IsPathImport(path string) bool {
	return strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}

// --- functions, types and traits

// Parameter is one function parameter. At most one of VarArgs (`*name`) and
// KwArgs (`**name`) is set.
type Parameter struct {
	Name    string
	Pos     token.Pos
	VarArgs bool
	KwArgs  bool
	Default Expression
}

func (p *Parameter) HasDefault() bool { return p.Default != nil }

// FunctionDefine is a named function, a type method, or a trait method. A
// trait's required method has a nil Body; every other body ends with an
// implicit `return nil`.
type FunctionDefine struct {
	statementMixin
	Name       string
	Parameters []*Parameter
	Body       []Statement
}

// FunctionLiteral is an anonymous function used as an expression, e.g.
// `var f = func(x) { return x + 1 }`. It captures its defining scope.
type FunctionLiteral struct {
	expressionMixin
	Parameters []*Parameter
	Body       []Statement
}

type TypeField struct {
	Name         string
	Pos          token.Pos
	DefaultValue Expression
}

func (f *TypeField) HasDefault() bool { return f.DefaultValue != nil }

type TypeDefine struct {
	statementMixin
	Name    string
	Fields  []*TypeField
	Methods []*FunctionDefine
	// Impls lists the type's impl blocks in declaration order.
	Impls []*ImplBlock
}

// AllMethods lists every method body the type declares: its ordinary methods
// followed by the methods of each impl block.
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

// ImplBlock is one type's implementation of one trait.
type ImplBlock struct {
	Trait   *TraitRef
	Methods []*FunctionDefine
}

// TraitDefine declares a trait. A method with a nil Body is required; one
// with a body is a default.
type TraitDefine struct {
	statementMixin
	Name    string
	Deps    []*TraitRef
	Methods []*FunctionDefine
}

// --- expressions

type Identifier struct {
	expressionMixin
	Name string
}

// Literal is a constant: Integer, Float, String, Bool or Nil.
type Literal struct {
	expressionMixin
	Value object.Object
}

type ListLiteral struct {
	expressionMixin
	Elements []Expression
}

type DictElement struct {
	Key   Expression
	Value Expression
}

type DictLiteral struct {
	expressionMixin
	Elements []*DictElement
}

// CallArgumentKind is the syntactic form of one call argument.
type CallArgumentKind int

const (
	CallArgumentPositional    CallArgumentKind = iota // f(x)
	CallArgumentStarred                               // f(*xs)
	CallArgumentKeyword                               // f(name=x)
	CallArgumentKeywordUnpack                         // f(**kw)
)

type CallArgument struct {
	Expr Expression
	Name string // keyword arguments only
	Kind CallArgumentKind
	// NamePos is the position of a keyword argument's name; MarkerPos that of
	// the operand of a `*` or `**` argument.
	NamePos   token.Pos
	MarkerPos token.Pos
}

// CallExpression is `Callee(Args)`. A call through a plain name has an
// *Identifier callee.
type CallExpression struct {
	expressionMixin
	Callee Expression
	Args   []CallArgument
}

// CalleeName returns the name of a call through a plain identifier.
func (c *CallExpression) CalleeName() (string, bool) {
	if ident, ok := c.Callee.(*Identifier); ok {
		return ident.Name, true
	}
	return "", false
}

// Operator spellings used by BinaryOperation and UnaryOperation.
const (
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

type UnaryOperation struct {
	expressionMixin
	Operand  Expression
	Operator string
}

type IndexExpression struct {
	expressionMixin
	Object Expression
	Index  Expression
}

type MemberExpression struct {
	expressionMixin
	Object   Expression
	Property string
}
