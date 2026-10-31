package interpreter

import (
	"fmt"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/token"
)

// stmtFn runs a compiled statement in the frame of its innermost frame scope.
// The returned value is only meaningful with ctrlReturn.
type stmtFn func(f *frame) (ctrl, object.Object, error)

// exprFn evaluates a compiled expression.
type exprFn func(f *frame) (object.Object, error)

// argsFn evaluates the arguments of a call.
type argsFn func(f *frame) (object.CallArgs, error)

// frameScope is the compile-time shape of one runtime frame: how many
// variable slots it holds and how much argument scratch space its call sites
// need. Blocks that do not need fresh storage share their enclosing frame
// scope, so their variables are flattened into it.
type frameScope struct {
	parent  *frameScope
	nvars   int
	temp    int
	maxTemp int
}

func (fs *frameScope) slot() int {
	i := fs.nvars
	fs.nvars++
	return i
}

// scope is one lexical scope. The module's top level is the global scope: a
// declaration there binds a global cell instead of a slot.
type scope struct {
	parent *scope
	fs     *frameScope
	names  map[string]int
	global bool
}

type compiler struct {
	g      *globals
	scope  *scope
	module *frameScope
	// noTemps is set while compiling an expression that runs in the frame
	// of an activation that may be in the middle of a call of its own (a
	// parameter or field default), so its call sites must not use that
	// frame's argument scratch space.
	noTemps bool

	funcs  map[*ast.FunctionDefine]*funcProto
	types  map[*ast.TypeDefine]*typeProto
	traits map[*ast.TraitDefine]*traitProto
}

func newCompiler(g *globals) *compiler {
	fs := &frameScope{}
	return &compiler{
		g:      g,
		module: fs,
		scope:  &scope{fs: fs, global: true},
		funcs:  make(map[*ast.FunctionDefine]*funcProto),
		types:  make(map[*ast.TypeDefine]*typeProto),
		traits: make(map[*ast.TraitDefine]*traitProto),
	}
}

// moduleFrame allocates the frame the module body runs in, sized once every
// top-level statement has been compiled.
func (c *compiler) moduleFrame() *frame {
	return newFrame(nil, c.module.nvars, c.module.maxTemp)
}

func (c *compiler) compileModule(stmts []ast.Statement) stmtFn {
	return c.statements(stmts)
}

// --- scopes and variables

func (c *compiler) push(ownFrame bool) {
	fs := c.scope.fs
	if ownFrame {
		fs = &frameScope{parent: fs}
	}
	c.scope = &scope{parent: c.scope, fs: fs, names: make(map[string]int)}
}

func (c *compiler) pop() { c.scope = c.scope.parent }

// varRef is a resolved variable: a global cell, or a slot hops frames up.
type varRef struct {
	cell *cell
	hops int
	idx  int
}

// lookup resolves a name from the current scope. A name no enclosing scope
// declares is global; the semantic checker guarantees it exists for checked
// code, and the REPL resolves it against the session at run time.
func (c *compiler) lookup(name string) varRef {
	for s := c.scope; s != nil; s = s.parent {
		if s.global {
			break
		}
		if idx, ok := s.names[name]; ok {
			hops := 0
			for fs := c.scope.fs; fs != s.fs; fs = fs.parent {
				hops++
			}
			return varRef{hops: hops, idx: idx}
		}
	}
	return varRef{cell: c.g.cell(name)}
}

// declare binds name in the current scope. Declaring a name the scope
// already has reuses its storage, as rebinding a name did before.
func (c *compiler) declare(name string) varRef {
	s := c.scope
	if s.global {
		return varRef{cell: c.g.cell(name)}
	}
	if idx, ok := s.names[name]; ok {
		return varRef{idx: idx}
	}
	idx := s.fs.slot()
	s.names[name] = idx
	return varRef{idx: idx}
}

func up(f *frame, hops int) *frame {
	for ; hops > 0; hops-- {
		f = f.parent
	}
	return f
}

func (r varRef) load(name string) exprFn {
	if cl := r.cell; cl != nil {
		return func(*frame) (object.Object, error) { return cl.get() }
	}
	idx := r.idx
	switch r.hops {
	case 0:
		return func(f *frame) (object.Object, error) {
			if v := f.slots[idx]; v != nil {
				return v, nil
			}
			return nil, undefinedName(name)
		}
	case 1:
		return func(f *frame) (object.Object, error) {
			if v := f.parent.slots[idx]; v != nil {
				return v, nil
			}
			return nil, undefinedName(name)
		}
	default:
		hops := r.hops
		return func(f *frame) (object.Object, error) {
			if v := up(f, hops).slots[idx]; v != nil {
				return v, nil
			}
			return nil, undefinedName(name)
		}
	}
}

// store compiles `target = value` for a resolved target.
func (r varRef) store(value exprFn) stmtFn {
	if cl := r.cell; cl != nil {
		return func(f *frame) (ctrl, object.Object, error) {
			v, err := value(f)
			if err != nil {
				return ctrlNone, nil, err
			}
			cl.value = v
			return ctrlNone, nil, nil
		}
	}
	idx := r.idx
	switch r.hops {
	case 0:
		return func(f *frame) (ctrl, object.Object, error) {
			v, err := value(f)
			if err != nil {
				return ctrlNone, nil, err
			}
			f.slots[idx] = v
			return ctrlNone, nil, nil
		}
	default:
		hops := r.hops
		return func(f *frame) (ctrl, object.Object, error) {
			v, err := value(f)
			if err != nil {
				return ctrlNone, nil, err
			}
			up(f, hops).slots[idx] = v
			return ctrlNone, nil, nil
		}
	}
}

// --- statements

func noop(*frame) (ctrl, object.Object, error) { return ctrlNone, nil, nil }

// statements compiles a statement list in the current scope. An error is
// tagged with the position of the statement it came from (see
// positionError), so tracebacks point at the failing line.
func (c *compiler) statements(stmts []ast.Statement) stmtFn {
	var fns []stmtFn
	var poss []token.Pos
	for _, stmt := range stmts {
		if fn := c.statement(stmt); fn != nil {
			fns = append(fns, fn)
			poss = append(poss, stmt.Position())
		}
	}
	switch len(fns) {
	case 0:
		return noop
	case 1:
		s0, p0 := fns[0], poss[0]
		return func(f *frame) (ctrl, object.Object, error) {
			ct, v, err := s0(f)
			if err != nil {
				return ctrlNone, nil, positionError(err, p0)
			}
			return ct, v, nil
		}
	}
	return func(f *frame) (ctrl, object.Object, error) {
		for i, s := range fns {
			ct, v, err := s(f)
			if err != nil {
				return ctrlNone, nil, positionError(err, poss[i])
			}
			if ct != ctrlNone {
				return ct, v, nil
			}
		}
		return ctrlNone, nil, nil
	}
}

// block compiles a lexical block. Goblin blocks follow Go's scoping rules:
// declarations are visible to nested blocks but do not escape the block.
func (c *compiler) block(stmts []ast.Statement) stmtFn {
	own := needsFrame(stmts, false)
	c.push(own)
	body := c.statements(stmts)
	fs := c.scope.fs
	c.pop()
	if !own {
		return body
	}
	nv, nt := fs.nvars, fs.maxTemp
	return func(f *frame) (ctrl, object.Object, error) {
		return body(newFrame(f, nv, nt))
	}
}

// needsFrame reports whether a block needs fresh variable storage on every
// entry: only when it declares something a function created inside it could
// capture. Otherwise its variables live in the enclosing frame. declares says
// the block binds a name before its statements (a loop or catch variable).
func needsFrame(stmts []ast.Statement, declares bool) bool {
	if !declares {
		for _, s := range stmts {
			switch s.(type) {
			case *ast.Declare, *ast.FunctionDefine:
				declares = true
			}
		}
	}
	return declares && stmtsHaveFunction(stmts)
}

func truthy(v object.Object) (bool, error) {
	if b, ok := v.(object.Bool); ok {
		return bool(b), nil
	}
	return v.ToBool()
}

// statement compiles one statement, or returns nil for one that does nothing
// when executed.
func (c *compiler) statement(stmt ast.Statement) stmtFn {
	switch s := stmt.(type) {
	case *ast.Declare:
		value := c.expr(s.Value)
		return c.declare(s.Name).store(value)

	case *ast.Assign:
		value := c.expr(s.Value)
		return c.lookup(s.Target).store(value)

	case *ast.SetIndex:
		objFn, idxFn, valFn := c.expr(s.Object), c.expr(s.Index), c.expr(s.Value)
		return func(f *frame) (ctrl, object.Object, error) {
			obj, err := objFn(f)
			if err != nil {
				return ctrlNone, nil, err
			}
			idx, err := idxFn(f)
			if err != nil {
				return ctrlNone, nil, err
			}
			v, err := valFn(f)
			if err != nil {
				return ctrlNone, nil, err
			}
			return ctrlNone, nil, object.SetIndex(obj, idx, v)
		}

	case *ast.SetAttr:
		objFn, valFn, name := c.expr(s.Object), c.expr(s.Value), s.Property
		return func(f *frame) (ctrl, object.Object, error) {
			obj, err := objFn(f)
			if err != nil {
				return ctrlNone, nil, err
			}
			v, err := valFn(f)
			if err != nil {
				return ctrlNone, nil, err
			}
			return ctrlNone, nil, object.SetAttr(obj, name, v)
		}

	case *ast.IfElse:
		cond := c.expr(s.Condition)
		ifBody := c.block(s.IfBody)
		var elseBody stmtFn
		if s.ElseBody != nil {
			elseBody = c.block(s.ElseBody)
		}
		return func(f *frame) (ctrl, object.Object, error) {
			v, err := cond(f)
			if err != nil {
				return ctrlNone, nil, err
			}
			t, err := truthy(v)
			if err != nil {
				return ctrlNone, nil, err
			}
			if t {
				return ifBody(f)
			}
			if elseBody != nil {
				return elseBody(f)
			}
			return ctrlNone, nil, nil
		}

	case *ast.While:
		cond := c.expr(s.Condition)
		body := c.block(s.Body)
		return func(f *frame) (ctrl, object.Object, error) {
			for {
				v, err := cond(f)
				if err != nil {
					return ctrlNone, nil, err
				}
				t, err := truthy(v)
				if err != nil {
					return ctrlNone, nil, err
				}
				if !t {
					return ctrlNone, nil, nil
				}
				ct, rv, err := body(f)
				if err != nil {
					return ctrlNone, nil, err
				}
				switch ct {
				case ctrlBreak:
					return ctrlNone, nil, nil
				case ctrlReturn:
					return ct, rv, nil
				}
			}
		}

	case *ast.For:
		return c.forLoop(s)

	case *ast.Break:
		return func(*frame) (ctrl, object.Object, error) { return ctrlBreak, nil, nil }

	case *ast.Continue:
		return func(*frame) (ctrl, object.Object, error) { return ctrlContinue, nil, nil }

	case *ast.Return:
		if s.Value == nil {
			return func(*frame) (ctrl, object.Object, error) { return ctrlReturn, object.Nil, nil }
		}
		value := c.expr(s.Value)
		return func(f *frame) (ctrl, object.Object, error) {
			v, err := value(f)
			if err != nil {
				return ctrlNone, nil, err
			}
			return ctrlReturn, v, nil
		}

	case *ast.Raise:
		value := c.expr(s.Value)
		return func(f *frame) (ctrl, object.Object, error) {
			v, err := value(f)
			if err != nil {
				return ctrlNone, nil, err
			}
			return ctrlNone, nil, object.Raise(v)
		}

	case *ast.TryCatch:
		return c.tryCatch(s)

	case *ast.FunctionDefine:
		if c.scope.global {
			// Hoisted when the module loads (see load), and defined again
			// when the statement runs.
			proto := c.function(s.Name, s.Name, s.Position(), s.Parameters, s.Body, false)
			c.funcs[s] = proto
			cl := c.g.cell(s.Name)
			return func(f *frame) (ctrl, object.Object, error) {
				cl.value = proto.function(f)
				return ctrlNone, nil, nil
			}
		}
		// Declared before the body is compiled, so it can call itself.
		target := c.declare(s.Name)
		proto := c.function(s.Name, s.Name, s.Position(), s.Parameters, s.Body, false)
		return target.store(func(f *frame) (object.Object, error) {
			return proto.function(f), nil
		})

	case *ast.TypeDefine:
		// Bound once, when the module is loaded (see load). Defining one
		// again when the statement runs would make the values created in
		// between belong to a different type.
		if c.scope.global {
			c.types[s] = c.typeDef(s)
		}
		return nil

	case *ast.TraitDefine:
		if c.scope.global {
			c.traits[s] = c.traitDef(s)
		}
		return nil

	case *ast.Import, *ast.Export:
		// Imports are resolved and bound by load before execution; exports
		// are markers consumed when a module is loaded.
		return nil

	case ast.Expression:
		// Bare expression statement (e.g. a call); evaluate for side effects.
		e := c.expr(s)
		return func(f *frame) (ctrl, object.Object, error) {
			_, err := e(f)
			return ctrlNone, nil, err
		}

	default:
		err := fmt.Errorf("interpreter: unsupported statement %T", stmt)
		return func(*frame) (ctrl, object.Object, error) { return ctrlNone, nil, err }
	}
}

func (c *compiler) forLoop(s *ast.For) stmtFn {
	iterFn := c.expr(s.Iterator)
	// Like a Go range clause, the iteration binding belongs to the loop,
	// not the surrounding block. When a closure in the body may capture it,
	// each iteration gets its own binding so closures do not all see the
	// final value.
	own := needsFrame(s.Body, true)
	c.push(own)
	idx := c.declare(s.Variable).idx
	body := c.block(s.Body)
	fs := c.scope.fs
	c.pop()

	loop := func(f *frame, items []object.Object, bind func(*frame, object.Object) *frame) (ctrl, object.Object, error) {
		for _, item := range items {
			ct, rv, err := body(bind(f, item))
			if err != nil {
				return ctrlNone, nil, err
			}
			switch ct {
			case ctrlBreak:
				return ctrlNone, nil, nil
			case ctrlReturn:
				return ct, rv, nil
			}
		}
		return ctrlNone, nil, nil
	}
	var bind func(*frame, object.Object) *frame
	if own {
		nv, nt := fs.nvars, fs.maxTemp
		bind = func(f *frame, item object.Object) *frame {
			nf := newFrame(f, nv, nt)
			nf.slots[idx] = item
			return nf
		}
	} else {
		bind = func(f *frame, item object.Object) *frame {
			f.slots[idx] = item
			return f
		}
	}
	return func(f *frame) (ctrl, object.Object, error) {
		iter, err := iterFn(f)
		if err != nil {
			return ctrlNone, nil, err
		}
		items, err := iter.Iter()
		if err != nil {
			return ctrlNone, nil, err
		}
		return loop(f, items, bind)
	}
}

func (c *compiler) tryCatch(s *ast.TryCatch) stmtFn {
	tryBody := c.block(s.TryBody)
	// The catch variable and the catch body share one scope.
	own := needsFrame(s.CatchBody, true)
	c.push(own)
	idx := c.declare(s.CatchVar).idx
	catchBody := c.statements(s.CatchBody)
	fs := c.scope.fs
	c.pop()
	nv, nt := fs.nvars, fs.maxTemp
	return func(f *frame) (ctrl, object.Object, error) {
		ct, v, err := tryBody(f)
		if err == nil {
			// Break, continue and return pass through untouched; only
			// genuine errors (raised Errors and runtime errors) are caught.
			return ct, v, nil
		}
		err, _ = takePosition(err, token.Pos{})
		caught := object.ErrorValue(err)
		if own {
			nf := newFrame(f, nv, nt)
			nf.slots[idx] = caught
			return catchBody(nf)
		}
		f.slots[idx] = caught
		return catchBody(f)
	}
}

// --- expressions

func (c *compiler) expr(expr ast.Expression) exprFn {
	switch e := expr.(type) {
	case *ast.Literal:
		v := e.Value
		return func(*frame) (object.Object, error) { return v, nil }

	case *ast.Identifier:
		return c.lookup(e.Name).load(e.Name)

	case *ast.ListLiteral:
		elems := c.exprs(e.Elements)
		return func(f *frame) (object.Object, error) {
			elements := make([]object.Object, len(elems))
			for i, el := range elems {
				v, err := el(f)
				if err != nil {
					return nil, err
				}
				elements[i] = v
			}
			return &object.List{Elements: elements}, nil
		}

	case *ast.DictLiteral:
		keys := make([]exprFn, len(e.Elements))
		values := make([]exprFn, len(e.Elements))
		for i, el := range e.Elements {
			keys[i] = c.expr(el.Key)
			values[i] = c.expr(el.Value)
		}
		return func(f *frame) (object.Object, error) {
			d := object.NewDict()
			for i := range keys {
				k, err := keys[i](f)
				if err != nil {
					return nil, err
				}
				v, err := values[i](f)
				if err != nil {
					return nil, err
				}
				if err := d.Set(k, v); err != nil {
					return nil, err
				}
			}
			return d, nil
		}

	case *ast.BinaryOperation:
		return c.binary(e)

	case *ast.UnaryOperation:
		return c.unary(e)

	case *ast.IndexExpression:
		objFn, idxFn := c.expr(e.Object), c.expr(e.Index)
		return func(f *frame) (object.Object, error) {
			obj, err := objFn(f)
			if err != nil {
				return nil, err
			}
			idx, err := idxFn(f)
			if err != nil {
				return nil, err
			}
			return obj.Index(idx)
		}

	case *ast.MemberExpression:
		objFn, name := c.expr(e.Object), e.Property
		return func(f *frame) (object.Object, error) {
			obj, err := objFn(f)
			if err != nil {
				return nil, err
			}
			return obj.GetAttr(name)
		}

	case *ast.FunctionLiteral:
		proto := c.function("<lambda>", "<lambda>", e.Position(), e.Parameters, e.Body, false)
		return func(f *frame) (object.Object, error) { return proto.function(f), nil }

	case *ast.CallExpression:
		// A method call resolves the receiver and dispatches in one step,
		// skipping the bound function that reading the attribute would
		// allocate. object.CallMethod falls back to GetAttr for anything
		// that is not a method, so a field holding a function still works.
		if member, ok := e.Callee.(*ast.MemberExpression); ok {
			recvFn, name := c.expr(member.Object), member.Property
			args := c.args(e.Args)
			return func(f *frame) (object.Object, error) {
				recv, err := recvFn(f)
				if err != nil {
					return nil, err
				}
				a, err := args(f)
				if err != nil {
					return nil, err
				}
				return object.CallMethod(recv, name, a)
			}
		}
		callee := c.expr(e.Callee)
		args := c.args(e.Args)
		return func(f *frame) (object.Object, error) {
			fn, err := callee(f)
			if err != nil {
				return nil, err
			}
			a, err := args(f)
			if err != nil {
				return nil, err
			}
			return object.Call(fn, a)
		}

	default:
		err := fmt.Errorf("interpreter: unsupported expression %T", expr)
		return func(*frame) (object.Object, error) { return nil, err }
	}
}

func (c *compiler) exprs(list []ast.Expression) []exprFn {
	fns := make([]exprFn, len(list))
	for i, e := range list {
		fns[i] = c.expr(e)
	}
	return fns
}

// args compiles a call's arguments. Plain positional arguments are evaluated
// into the call site's window of the frame's temps and passed without
// allocating: object.CallArgs is only valid for the duration of the call, and
// a callee that keeps the arguments copies them.
func (c *compiler) args(args []ast.CallArgument) argsFn {
	positional := true
	for _, a := range args {
		if a.Kind != ast.CallArgumentPositional {
			positional = false
		}
	}
	if positional && !c.noTemps {
		n := len(args)
		if n == 0 {
			return func(*frame) (object.CallArgs, error) { return object.CallArgs{}, nil }
		}
		fs := c.scope.fs
		base := fs.temp
		// Reserve the window before compiling the arguments, so call sites
		// nested inside them take the space above it.
		fs.temp += n
		if fs.temp > fs.maxTemp {
			fs.maxTemp = fs.temp
		}
		fns := make([]exprFn, n)
		for i, a := range args {
			fns[i] = c.expr(a.Expr)
		}
		fs.temp -= n
		end := base + n
		if n == 1 {
			a0 := fns[0]
			return func(f *frame) (object.CallArgs, error) {
				v, err := a0(f)
				if err != nil {
					return object.CallArgs{}, err
				}
				buf := f.temps[base:end:end]
				buf[0] = v
				return object.CallArgs{Positional: buf}, nil
			}
		}
		return func(f *frame) (object.CallArgs, error) {
			buf := f.temps[base:end:end]
			for i, a := range fns {
				v, err := a(f)
				if err != nil {
					return object.CallArgs{}, err
				}
				buf[i] = v
			}
			return object.CallArgs{Positional: buf}, nil
		}
	}

	kinds := make([]ast.CallArgumentKind, len(args))
	names := make([]string, len(args))
	fns := make([]exprFn, len(args))
	for i, a := range args {
		kinds[i], names[i], fns[i] = a.Kind, a.Name, c.expr(a.Expr)
	}
	return func(f *frame) (object.CallArgs, error) {
		var call object.CallArgs
		for i, fn := range fns {
			v, err := fn(f)
			if err != nil {
				return call, err
			}
			switch kinds[i] {
			case ast.CallArgumentPositional:
				call.Positional = append(call.Positional, v)
			case ast.CallArgumentStarred:
				items, err := v.Iter()
				if err != nil {
					return call, err
				}
				call.Positional = append(call.Positional, items...)
			case ast.CallArgumentKeyword:
				if err := call.AddKeyword(names[i], v); err != nil {
					return call, err
				}
			case ast.CallArgumentKeywordUnpack:
				if err := call.UnpackKeywords(v); err != nil {
					return call, err
				}
			default:
				return call, fmt.Errorf("interpreter: unsupported call argument kind %v", kinds[i])
			}
		}
		return call, nil
	}
}

// outOfBand compiles an expression that runs in a frame it does not own (a
// default value, evaluated during somebody else's call), so it must leave
// that frame's temps alone.
func (c *compiler) outOfBand(e ast.Expression) exprFn {
	prev := c.noTemps
	c.noTemps = true
	fn := c.expr(e)
	c.noTemps = prev
	return fn
}

func (c *compiler) binary(e *ast.BinaryOperation) exprFn {
	lhsFn, rhsFn := c.expr(e.LHS), c.expr(e.RHS)

	// Short-circuit logical operators: the RHS is only evaluated when the LHS
	// does not already determine the result, so side effects (and errors) in
	// the skipped operand never run. The value of the expression is the
	// operand that decided it, not a coerced Bool, which is what makes
	// `x || fallback` usable as a default.
	if e.Operator == ast.And || e.Operator == ast.Or {
		isAnd := e.Operator == ast.And
		return func(f *frame) (object.Object, error) {
			lhs, err := lhsFn(f)
			if err != nil {
				return nil, err
			}
			t, err := truthy(lhs)
			if err != nil {
				return nil, err
			}
			if t != isAnd {
				return lhs, nil
			}
			return rhsFn(f)
		}
	}

	var arith func(a, b object.Object) (object.Object, error)
	var cmp func(a, b object.Object) (bool, error)
	switch e.Operator {
	case ast.Add:
		arith = object.Add
	case ast.Minus:
		arith = object.Minus
	case ast.Multiply:
		arith = object.Multiply
	case ast.Divide:
		arith = object.Divide
	case ast.Modulo:
		arith = object.Modulo
	case ast.Equal:
		cmp = object.Equals
	case ast.NotEqual:
		cmp = object.NotEquals
	case ast.LessThan:
		cmp = object.Less
	case ast.LessOrEqual:
		cmp = object.LessEqual
	case ast.GreaterThan:
		cmp = object.Greater
	case ast.GreaterOrEqual:
		cmp = object.GreaterEqual
	default:
		op := e.Operator
		return func(f *frame) (object.Object, error) {
			if _, err := lhsFn(f); err != nil {
				return nil, err
			}
			if _, err := rhsFn(f); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("interpreter: unknown operator %q", op)
		}
	}
	if arith != nil {
		return func(f *frame) (object.Object, error) {
			lhs, err := lhsFn(f)
			if err != nil {
				return nil, err
			}
			rhs, err := rhsFn(f)
			if err != nil {
				return nil, err
			}
			return arith(lhs, rhs)
		}
	}
	return func(f *frame) (object.Object, error) {
		lhs, err := lhsFn(f)
		if err != nil {
			return nil, err
		}
		rhs, err := rhsFn(f)
		if err != nil {
			return nil, err
		}
		r, err := cmp(lhs, rhs)
		return object.Bool(r), err
	}
}

func (c *compiler) unary(e *ast.UnaryOperation) exprFn {
	operandFn := c.expr(e.Operand)
	var op func(object.Object) (object.Object, error)
	switch e.Operator {
	case ast.Not:
		op = object.Not
	case ast.Add:
		op = object.Positive
	case ast.Minus:
		op = object.Negate
	default:
		unknown := e.Operator
		op = func(object.Object) (object.Object, error) {
			return nil, fmt.Errorf("interpreter: unknown unary operator %q", unknown)
		}
	}
	return func(f *frame) (object.Object, error) {
		operand, err := operandFn(f)
		if err != nil {
			return nil, err
		}
		return op(operand)
	}
}

// --- capture analysis

// stmtsHaveFunction reports whether a function literal or definition (types
// and traits included, for their methods) appears anywhere in stmts, the only
// way a block's variables can outlive one entry of the block.
func stmtsHaveFunction(stmts []ast.Statement) bool {
	return ast.Any(stmts, false, ast.OpensScope)
}
