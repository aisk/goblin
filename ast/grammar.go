package ast

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/token"
)

// This file holds the constructors goblin.bnf's reduce actions call. gocc
// passes every symbol as an untyped value (a *token.Token for terminals, the
// constructed node or list for non-terminals, nil for an empty production), so
// these functions take and return `any`. Nothing outside the generated parser
// should call them.

// statements converts a reduced Statements node to a slice. An empty block
// (and an empty module) reduces through `Statements : empty`, which yields a
// nil any rather than an empty slice, so every constructor taking a block
// goes through here instead of asserting the type directly.
func statements(x any) []Statement {
	if x == nil {
		return nil
	}
	return x.([]Statement)
}

// listOf converts an optional reduced list to its slice type.
func listOf[T any](x any) []T {
	if x == nil {
		return nil
	}
	return x.([]T)
}

func tokenOf(x any) *token.Token { return x.(*token.Token) }
func lit(x any) string           { return string(tokenOf(x).Lit) }

// nilLiteral is the value of a bare `return` and of the implicit return every
// function body ends with.
func nilLiteral() *Literal { return &Literal{Value: object.Nil} }

// withImplicitReturn appends the `return nil` every function body ends with,
// so the backends never special-case falling off the end of a body.
func withImplicitReturn(body []Statement) []Statement {
	return append(body, &Return{Value: nilLiteral()})
}

// positionOf returns the position of a node or token, or the zero position.
func positionOf(v any) token.Pos {
	switch n := v.(type) {
	case Statement:
		return n.Position()
	case *token.Token:
		return n.Pos
	}
	return token.Pos{}
}

// --- lists

func NewStatementList(x any) (any, error)       { return []Statement{x.(Statement)}, nil }
func AppendStatementList(l, x any) (any, error) { return append(l.([]Statement), x.(Statement)), nil }

func NewExpressionList(x any) (any, error) { return []Expression{x.(Expression)}, nil }
func AppendExpressionList(l, x any) (any, error) {
	return append(l.([]Expression), x.(Expression)), nil
}

func NewCallArgumentList(x any) (any, error) { return []CallArgument{*x.(*CallArgument)}, nil }
func AppendCallArgumentList(l, x any) (any, error) {
	return append(l.([]CallArgument), *x.(*CallArgument)), nil
}

func NewDictElementList(x any) (any, error) { return []*DictElement{x.(*DictElement)}, nil }
func AppendDictElementList(l, x any) (any, error) {
	return append(l.([]*DictElement), x.(*DictElement)), nil
}

func NewParameterList(x any) (any, error) { return []*Parameter{x.(*Parameter)}, nil }
func AppendParameterList(l, x any) (any, error) {
	return append(l.([]*Parameter), x.(*Parameter)), nil
}

func NewTypeFieldList(x any) (any, error) { return []*TypeField{x.(*TypeField)}, nil }
func AppendTypeFieldList(l, x any) (any, error) {
	return append(l.([]*TypeField), x.(*TypeField)), nil
}

func NewTypeMethodList(x any) (any, error) { return []*FunctionDefine{x.(*FunctionDefine)}, nil }
func AppendTypeMethodList(l, x any) (any, error) {
	return append(l.([]*FunctionDefine), x.(*FunctionDefine)), nil
}

// A type body mixes methods and impl blocks; NewTypeDefine sorts them.
func NewTypeMemberList(x any) (any, error)       { return []any{x}, nil }
func AppendTypeMemberList(l, x any) (any, error) { return append(l.([]any), x), nil }

// --- module and statements

func NewModule(x any) (any, error) {
	return &Module{Name: "main", Body: statements(x)}, nil
}

func NewDeclare(name, value any) (any, error) {
	return &Declare{statementMixin: stmtAt(tokenOf(name).Pos), Name: lit(name), Value: value.(Expression)}, nil
}

// NewSetAssign builds the statement for `target = value`: an Assign, SetIndex
// or SetAttr depending on the target's shape. Other targets are rejected.
func NewSetAssign(target, value any) (any, error) {
	v := value.(Expression)
	switch t := target.(type) {
	case *Identifier:
		return &Assign{statementMixin: stmtAt(t.Position()), Target: t.Name, Value: v}, nil
	case *IndexExpression:
		return &SetIndex{statementMixin: stmtAt(t.Position()), Object: t.Object, Index: t.Index, Value: v}, nil
	case *MemberExpression:
		return &SetAttr{statementMixin: stmtAt(t.Position()), Object: t.Object, Property: t.Property, Value: v}, nil
	}
	return nil, fmt.Errorf("cannot assign to this expression")
}

// NewDestructureDeclare builds `var first, rest... = value`.
func NewDestructureDeclare(first, rest, value any) (any, error) {
	names := append([]*token.Token{tokenOf(first)}, listOf[*token.Token](rest)...)
	d := &Destructure{statementMixin: stmtAt(names[0].Pos), Value: value.(Expression)}
	for i, name := range names {
		d.Stores = append(d.Stores, &Declare{
			statementMixin: stmtAt(name.Pos),
			Name:           string(name.Lit),
			Value:          &Unpacked{expressionMixin: exprAt(name.Pos), Index: i},
		})
	}
	return d, nil
}

// NewDestructureAssign builds `first, rest... = value`. Each target may be
// anything a plain assignment accepts.
func NewDestructureAssign(first, rest, value any) (any, error) {
	targets := append([]Expression{first.(Expression)}, listOf[Expression](rest)...)
	d := &Destructure{statementMixin: stmtAt(targets[0].Position()), Value: value.(Expression)}
	for i, target := range targets {
		store, err := NewSetAssign(target, &Unpacked{expressionMixin: exprAt(target.Position()), Index: i})
		if err != nil {
			return nil, err
		}
		d.Stores = append(d.Stores, store.(Statement))
	}
	return d, nil
}

func NewNameList(x any) (any, error) { return []*token.Token{tokenOf(x)}, nil }
func AppendNameList(l, x any) (any, error) {
	return append(l.([]*token.Token), tokenOf(x)), nil
}

// NewIf builds an if statement. elseBody is a statement list, a nested
// *IfElse for `else if`, or nil.
func NewIf(cond, body, elseBody any) (any, error) {
	stmt := &IfElse{
		statementMixin: stmtAt(positionOf(cond)),
		Condition:      cond.(Expression),
		IfBody:         statements(body),
	}
	if chained, ok := elseBody.(*IfElse); ok {
		stmt.ElseBody = []Statement{chained}
	} else {
		stmt.ElseBody = statements(elseBody)
	}
	return stmt, nil
}

func NewWhile(cond, body any) (any, error) {
	return &While{statementMixin: stmtAt(positionOf(cond)), Condition: cond.(Expression), Body: statements(body)}, nil
}

func NewFor(variable, iterator, body any) (any, error) {
	return &For{
		statementMixin: stmtAt(tokenOf(variable).Pos),
		Variable:       lit(variable),
		Iterator:       iterator.(Expression),
		Body:           statements(body),
	}, nil
}

// NewDestructureFor builds `for first, rest... in iterator { body }`. The
// loop binds each element to the hidden DestructuredItem variable, and the
// body starts with the statement that destructures it into the names, so
// the backends see an ordinary loop.
func NewDestructureFor(first, rest, iterator, body any) (any, error) {
	pos := tokenOf(first).Pos
	unpack, err := NewDestructureDeclare(first, rest, &Identifier{expressionMixin: exprAt(pos), Name: DestructuredItem})
	if err != nil {
		return nil, err
	}
	return &For{
		statementMixin: stmtAt(pos),
		Variable:       DestructuredItem,
		Iterator:       iterator.(Expression),
		Body:           append([]Statement{unpack.(Statement)}, statements(body)...),
	}, nil
}

func NewBreak(x any) (any, error) { return &Break{statementMixin: stmtAt(tokenOf(x).Pos)}, nil }
func NewContinue(x any) (any, error) {
	return &Continue{statementMixin: stmtAt(tokenOf(x).Pos)}, nil
}

func NewReturn(x any) (any, error) {
	return &Return{statementMixin: stmtAt(positionOf(x)), Value: x.(Expression)}, nil
}

// NewReturnNil builds a bare `return`, which returns nil.
func NewReturnNil(x any) (any, error) {
	return &Return{statementMixin: stmtAt(tokenOf(x).Pos), Value: nilLiteral()}, nil
}

func NewRaise(x any) (any, error) {
	return &Raise{statementMixin: stmtAt(positionOf(x)), Value: x.(Expression)}, nil
}

func NewDefer(deferTok, call any) (any, error) {
	c, ok := call.(*CallExpression)
	if !ok {
		return nil, fmt.Errorf("defer needs a function call")
	}
	return &Defer{statementMixin: stmtAt(tokenOf(deferTok).Pos), Call: c}, nil
}

// NewTry builds a try statement. A clause after a catch-all one could never
// run, which is reported at that clause.
func NewTry(tryTok, tryBody, clauses any) (any, error) {
	list := clauses.([]*CatchClause)
	for i, c := range list[:len(list)-1] {
		if c.Kind == nil {
			return nil, &SyntaxError{Pos: list[i+1].Pos, Message: "catch after a catch-all clause can never run"}
		}
	}
	return &TryCatch{
		statementMixin: stmtAt(tokenOf(tryTok).Pos),
		TryBody:        statements(tryBody),
		Clauses:        list,
	}, nil
}

func NewCatchList(clause any) (any, error) {
	return []*CatchClause{clause.(*CatchClause)}, nil
}

func AppendCatchList(list, clause any) (any, error) {
	return append(list.([]*CatchClause), clause.(*CatchClause)), nil
}

// NewCatch builds one catch clause; kind is nil for a catch-all clause.
func NewCatch(catchTok, name, kind, body any) (any, error) {
	c := &CatchClause{
		Pos:  tokenOf(catchTok).Pos,
		Var:  lit(name),
		Body: statements(body),
	}
	if kind != nil {
		c.Kind = kind.(Expression)
	}
	return c, nil
}

func NewExport(x any) (any, error) {
	return &Export{statementMixin: stmtAt(tokenOf(x).Pos), Name: lit(x)}, nil
}

func NewImport(x any) (any, error) {
	raw := lit(x)
	path := raw[1 : len(raw)-1] // strip quotes
	name := path[strings.LastIndex(path, "/")+1:]
	return &Import{statementMixin: stmtAt(tokenOf(x).Pos), Name: name, Path: path}, nil
}

// --- functions and types

func NewRequiredParameter(x any) (any, error) {
	return &Parameter{Name: lit(x), Pos: tokenOf(x).Pos}, nil
}

func NewDefaultParameter(name, value any) (any, error) {
	return &Parameter{Name: lit(name), Pos: tokenOf(name).Pos, Default: value.(Expression)}, nil
}

func NewVarArgsParameter(x any) (any, error) {
	return &Parameter{Name: lit(x), Pos: tokenOf(x).Pos, VarArgs: true}, nil
}

func NewKwArgsParameter(x any) (any, error) {
	return &Parameter{Name: lit(x), Pos: tokenOf(x).Pos, KwArgs: true}, nil
}

func NewFunctionDefine(name, params, body any) (any, error) {
	return &FunctionDefine{
		statementMixin: stmtAt(tokenOf(name).Pos),
		Name:           lit(name),
		Parameters:     listOf[*Parameter](params),
		Body:           withImplicitReturn(statements(body)),
	}, nil
}

func NewFunctionLiteral(fn, params, body any) (any, error) {
	return &FunctionLiteral{
		expressionMixin: exprAt(tokenOf(fn).Pos),
		Parameters:      listOf[*Parameter](params),
		Body:            withImplicitReturn(statements(body)),
	}, nil
}

func NewRequiredTypeField(x any) (any, error) {
	return &TypeField{Name: lit(x), Pos: tokenOf(x).Pos}, nil
}

func NewDefaultTypeField(name, value any) (any, error) {
	return &TypeField{Name: lit(name), Pos: tokenOf(name).Pos, DefaultValue: value.(Expression)}, nil
}

func NewTypeDefine(name, fields, members any) (any, error) {
	def := &TypeDefine{
		statementMixin: stmtAt(tokenOf(name).Pos),
		Name:           lit(name),
		Fields:         listOf[*TypeField](fields),
	}
	for _, member := range listOf[any](members) {
		switch m := member.(type) {
		case *FunctionDefine:
			def.Methods = append(def.Methods, m)
		case *ImplBlock:
			def.Impls = append(def.Impls, m)
		}
	}
	return def, nil
}

func NewImplBlock(name, methods any) (any, error) {
	return &ImplBlock{Protocol: lit(name), Pos: tokenOf(name).Pos, Methods: listOf[*FunctionDefine](methods)}, nil
}

// --- expressions

func NewIdentifier(x any) (any, error) {
	return &Identifier{expressionMixin: exprAt(tokenOf(x).Pos), Name: lit(x)}, nil
}

// NewIntegerLiteral reads a decimal, 0x hexadecimal, 0o octal or 0b binary
// integer literal, with optional '_' separators between digits. A decimal
// literal with a leading zero is an error rather than either a decimal or an
// octal number, so `0644` cannot silently mean the wrong thing.
func NewIntegerLiteral(x any) (any, error) {
	l, err := integerLiteral(x)
	if err != nil {
		// Reported at the start of the literal, not at the token after it.
		return nil, &SyntaxError{Pos: tokenOf(x).Pos, Message: err.Error()}
	}
	return l, nil
}

func integerLiteral(x any) (*Literal, error) {
	text := lit(x)
	digits, base, kind := text, 10, "decimal"
	if len(text) > 1 && text[0] == '0' {
		switch text[1] {
		case 'x':
			digits, base, kind = text[2:], 16, "hexadecimal"
		case 'o':
			digits, base, kind = text[2:], 8, "octal"
		case 'b':
			digits, base, kind = text[2:], 2, "binary"
		}
	}
	if err := checkSeparators(text, digits, base); err != nil {
		return nil, err
	}
	digits = strings.ReplaceAll(digits, "_", "")
	if base == 10 && len(digits) > 1 && digits[0] == '0' {
		octal := strings.TrimLeft(digits, "0")
		if octal == "" {
			octal = "0"
		}
		return nil, fmt.Errorf("invalid integer literal %s: a decimal number cannot start with 0 (write 0o%s for an octal number)", text, octal)
	}
	d, err := strconv.ParseInt(digits, base, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			// 2**63 is the magnitude of the smallest Int: written after a
			// unary minus it is that Int (see NewUnaryOperation), anywhere
			// else CheckLiterals reports it.
			if u, uerr := strconv.ParseUint(digits, base, 64); uerr == nil && u == 1<<63 {
				return &Literal{expressionMixin: exprAt(tokenOf(x).Pos), Value: object.Integer(math.MinInt64), minIntText: text}, nil
			}
			return nil, fmt.Errorf(errFmtIntRange, text)
		}
		return nil, fmt.Errorf("invalid digit in %s literal %s", kind, text)
	}
	return &Literal{expressionMixin: exprAt(tokenOf(x).Pos), Value: object.Integer(d)}, nil
}

// NewFloatLiteral reads a decimal floating-point literal with an optional
// exponent (`1.5`, `1e9`, `1.5e-3`) and '_' separators between digits.
func NewFloatLiteral(x any) (any, error) {
	l, err := floatLiteral(x)
	if err != nil {
		return nil, &SyntaxError{Pos: tokenOf(x).Pos, Message: err.Error()}
	}
	return l, nil
}

func floatLiteral(x any) (*Literal, error) {
	text := lit(x)
	if err := checkSeparators(text, text, 10); err != nil {
		return nil, err
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(text, "_", ""), 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return nil, fmt.Errorf("float literal %s is out of range for Float", text)
		}
		return nil, fmt.Errorf("invalid float literal %s", text)
	}
	return &Literal{expressionMixin: exprAt(tokenOf(x).Pos), Value: object.Float(f)}, nil
}

// checkSeparators reports an error unless every '_' in digits sits between
// two digits of the given base.
func checkSeparators(text, digits string, base int) error {
	isDigit := func(i int) bool {
		if i < 0 || i >= len(digits) {
			return false
		}
		d := digits[i]
		switch {
		case d >= '0' && d <= '9':
			return int(d-'0') < base
		case d >= 'a' && d <= 'f', d >= 'A' && d <= 'F':
			return base == 16
		}
		return false
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] == '_' && (!isDigit(i-1) || !isDigit(i+1)) {
			return fmt.Errorf("invalid number literal %s: '_' must separate two digits", text)
		}
	}
	return nil
}

func NewTrueLiteral() (any, error)  { return &Literal{Value: object.True}, nil }
func NewFalseLiteral() (any, error) { return &Literal{Value: object.False}, nil }
func NewNilLiteral() (any, error)   { return nilLiteral(), nil }

func NewListLiteral(x any) (any, error) {
	elements := listOf[Expression](x)
	var pos token.Pos
	if len(elements) > 0 {
		pos = elements[0].Position()
	}
	return &ListLiteral{expressionMixin: exprAt(pos), Elements: elements}, nil
}

func NewDictElement(key, value any) (any, error) {
	return &DictElement{Key: key.(Expression), Value: value.(Expression)}, nil
}

func NewDictLiteral(x any) (any, error) {
	elements := listOf[*DictElement](x)
	var pos token.Pos
	if len(elements) > 0 {
		pos = elements[0].Key.Position()
	}
	return &DictLiteral{expressionMixin: exprAt(pos), Elements: elements}, nil
}

func NewPositionalArgument(x any) (any, error) {
	return &CallArgument{Expr: x.(Expression), Kind: CallArgumentPositional}, nil
}

func NewStarredArgument(x any) (any, error) {
	return &CallArgument{Expr: x.(Expression), Kind: CallArgumentStarred, MarkerPos: positionOf(x)}, nil
}

func NewKeywordArgument(name, expr any) (any, error) {
	return &CallArgument{Expr: expr.(Expression), Name: lit(name), NamePos: tokenOf(name).Pos, Kind: CallArgumentKeyword}, nil
}

func NewKeywordUnpackArgument(x any) (any, error) {
	return &CallArgument{Expr: x.(Expression), Kind: CallArgumentKeywordUnpack, MarkerPos: positionOf(x)}, nil
}

func NewCallExpression(callee, args any) (any, error) {
	return &CallExpression{
		expressionMixin: exprAt(positionOf(callee)),
		Callee:          callee.(Expression),
		Args:            listOf[CallArgument](args),
	}, nil
}

func NewBinaryOperation(lhs, operator, rhs any) (any, error) {
	op := operator.(string)
	switch op {
	case Add, Minus, Multiply, Divide, Modulo, And, Or, Equal, NotEqual, LessThan, GreaterThan, LessOrEqual, GreaterOrEqual:
	default:
		return nil, fmt.Errorf("invalid operator: '%s'", op)
	}
	left := lhs.(Expression)
	return &BinaryOperation{expressionMixin: exprAt(left.Position()), LHS: left, RHS: rhs.(Expression), Operator: op}, nil
}

func NewUnaryOperation(operator, operand any) (any, error) {
	op := lit(operator)
	switch op {
	case Add, Minus, Not:
	default:
		return nil, fmt.Errorf("invalid unary operator: '%s'", op)
	}
	if l, ok := operand.(*Literal); ok && op == Minus && l.minIntText != "" {
		return &Literal{expressionMixin: exprAt(tokenOf(operator).Pos), Value: l.Value}, nil
	}
	return &UnaryOperation{expressionMixin: exprAt(tokenOf(operator).Pos), Operand: operand.(Expression), Operator: op}, nil
}

func NewIndexExpression(obj, idx any) (any, error) {
	o := obj.(Expression)
	return &IndexExpression{expressionMixin: exprAt(o.Position()), Object: o, Index: idx.(Expression)}, nil
}

// sliceBounds carries the two optional bounds of a slice from its own
// production to NewSliceExpression.
type sliceBounds struct{ low, high Expression }

func NewSliceBounds(low, high any) (any, error) {
	b := &sliceBounds{}
	if low != nil {
		b.low = low.(Expression)
	}
	if high != nil {
		b.high = high.(Expression)
	}
	return b, nil
}

func NewSliceExpression(obj, bounds any) (any, error) {
	o, b := obj.(Expression), bounds.(*sliceBounds)
	return &SliceExpression{expressionMixin: exprAt(o.Position()), Object: o, Low: b.low, High: b.high}, nil
}

func NewMemberExpression(obj, prop any) (any, error) {
	return &MemberExpression{expressionMixin: exprAt(tokenOf(prop).Pos), Object: obj.(Expression), Property: lit(prop)}, nil
}

const errFmtIntRange = "integer literal %s is out of range for Int"

// CheckLiterals reports an integer literal 9223372036854775808 (2**63, in
// any base) that is not the operand of a unary minus: only -9223372036854775808
// is an Int. The parser cannot tell while it reads the literal, so the check
// runs on the finished tree.
func CheckLiterals(stmts []Statement) error {
	var err error
	InspectList(stmts, func(n Statement) bool {
		if l, ok := n.(*Literal); ok && l.minIntText != "" && err == nil {
			err = &SyntaxError{Pos: l.Position(), Message: fmt.Sprintf(errFmtIntRange, l.minIntText)}
		}
		return err == nil
	})
	return err
}
