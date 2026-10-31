// Package interpreter provides a tree-walking interpreter for Goblin.
//
// Unlike the transpiler, which emits Go source code, the interpreter runs the
// AST directly using the runtime types in the object package. Both share the
// same object.Object value system, so arithmetic, comparison, iteration and
// built-in functions behave identically.
//
// Before running, a module is compiled into a tree of Go closures (see
// compile.go). Compilation resolves every name once: a local variable becomes
// an index into its function's frame, a module-level name becomes a pointer
// to its global cell. Running a function then costs one frame allocation and
// no name lookups.
package interpreter

import (
	"path/filepath"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/token"
)

// frame holds the local variables of one activation: a function call, a
// module run, or a block whose variables a closure may capture (each entry
// of such a block needs fresh storage, see needsFrame). parent is the frame
// of the lexically enclosing scope, so a variable hops a fixed, statically
// known number of parents to reach its slot.
//
// temps is the activation's scratch space for call arguments. Each call site
// owns a fixed window of it, so a positional call allocates no argument
// slice. Only the activation that owns the frame ever touches its temps:
// recursion and goroutines always run in a frame of their own.
type frame struct {
	parent *frame
	slots  []object.Object
	temps  []object.Object
}

// Frames up to a few dozen slots are allocated together with their storage,
// one allocation per activation.
type (
	frame4 struct {
		frame
		arr [4]object.Object
	}
	frame8 struct {
		frame
		arr [8]object.Object
	}
	frame16 struct {
		frame
		arr [16]object.Object
	}
	frame32 struct {
		frame
		arr [32]object.Object
	}
)

func newFrame(parent *frame, nvars, ntemps int) *frame {
	n := nvars + ntemps
	var f *frame
	var buf []object.Object
	switch {
	case n == 0:
		f = &frame{}
	case n <= 4:
		s := &frame4{}
		f, buf = &s.frame, s.arr[:n]
	case n <= 8:
		s := &frame8{}
		f, buf = &s.frame, s.arr[:n]
	case n <= 16:
		s := &frame16{}
		f, buf = &s.frame, s.arr[:n]
	case n <= 32:
		s := &frame32{}
		f, buf = &s.frame, s.arr[:n]
	default:
		f = &frame{}
		buf = make([]object.Object, n)
	}
	f.parent = parent
	if n > 0 {
		f.slots = buf[:nvars:nvars]
		f.temps = buf[nvars:]
	}
	return f
}

// cell is the storage of one module-level name. Compiled code holds the
// pointer, so reading a global is a load and a nil check. A cell whose value
// is nil is not bound yet; reading it falls back to the built-in of the same
// name, the order the old scope chain resolved names in.
//
// Distinct globals live in distinct cells, so goroutines reading one global
// while the main goroutine binds another never touch shared memory. Racing
// on the same global is the program's own data race, as for any shared
// mutable value.
type cell struct {
	name    string
	value   object.Object
	builtin object.Object
}

func (c *cell) get() (object.Object, error) {
	if v := c.value; v != nil {
		return v, nil
	}
	if c.builtin != nil {
		return c.builtin, nil
	}
	return nil, undefinedName(c.name)
}

func undefinedName(name string) error {
	return object.NewNameError("undefined: %s", name)
}

// globals is a module's top-level scope. The map is only consulted while
// compiling and loading, always on the goroutine doing so; running code
// reaches cells through the pointers compiled into it.
type globals struct {
	cells map[string]*cell
}

func newGlobals() *globals {
	return &globals{cells: make(map[string]*cell)}
}

// cell returns the cell for name, creating an unbound one on first use. A
// name used before it is declared (a forward reference from a function body,
// or anything in the unchecked REPL) shares the cell its later declaration
// binds.
func (g *globals) cell(name string) *cell {
	c, ok := g.cells[name]
	if !ok {
		c = &cell{name: name, builtin: extension.BuiltinsModule.Members[name]}
		g.cells[name] = c
	}
	return c
}

// Get returns the value bound to a module-level name, ignoring built-ins.
func (g *globals) Get(name string) (object.Object, bool) {
	if c, ok := g.cells[name]; ok && c.value != nil {
		return c.value, true
	}
	return nil, false
}

// Define binds a module-level name.
func (g *globals) Define(name string, v object.Object) {
	g.cell(name).value = v
}

// resolve looks a name up the way an identifier does: module bindings first,
// then built-ins.
func (g *globals) resolve(name string) (object.Object, error) {
	if c, ok := g.cells[name]; ok {
		return c.get()
	}
	if b, ok := extension.BuiltinsModule.Members[name]; ok {
		return b, nil
	}
	return nil, undefinedName(name)
}

// names lists the bound module-level names.
func (g *globals) names() []string {
	result := make([]string, 0, len(g.cells))
	for name, c := range g.cells {
		if c.value != nil {
			result = append(result, name)
		}
	}
	return result
}

// Control flow leaving a statement: a statement either finishes normally or
// breaks, continues or returns, and that travels up as a plain value rather
// than an error.
type ctrl uint8

const (
	ctrlNone ctrl = iota
	ctrlBreak
	ctrlContinue
	ctrlReturn
)

// The errors a stray break, continue or return turns into. Only unchecked
// REPL input can produce them; the semantic checker rejects them in files.
type breakSignal struct{}

func (breakSignal) Error() string { return "break outside loop" }

type continueSignal struct{}

func (continueSignal) Error() string { return "continue outside loop" }

type returnSignal struct{}

func (returnSignal) Error() string { return "return outside function" }

func strayCtrl(c ctrl) error {
	switch c {
	case ctrlBreak:
		return breakSignal{}
	case ctrlContinue:
		return continueSignal{}
	default:
		return returnSignal{}
	}
}

// Run interprets a parsed module. sourcePath is the path of the source file,
// used to resolve relative imports. scriptArgs are forwarded to os.argv() as
// elements after sourcePath (index 0).
func Run(mod *ast.Module, sourcePath string, scriptArgs ...string) (err error) {
	// A Go-side panic in the runtime must not kill the process with a raw Go
	// stack trace; surface it as an internal error instead.
	defer func() {
		if r := recover(); r != nil {
			err = object.NewInternalError("internal error: %v", r)
		}
	}()

	argv := make([]string, 1+len(scriptArgs))
	argv[0] = sourcePath
	copy(argv[1:], scriptArgs)

	_, err = runModule(mod, filepath.Dir(sourcePath), moduleName(sourcePath), object.NewRegistry(), argv)
	return err
}

// runModule compiles, loads and runs a module body in a fresh global scope,
// returning that scope so an importer can read the exports.
func runModule(mod *ast.Module, baseDir, module string, reg *object.Registry, argv []string) (*globals, error) {
	g := newGlobals()
	c := newCompiler(g)
	body := c.compileModule(mod.Body)
	f := c.moduleFrame()

	// Resolve imports and hoist top-level function/type definitions so
	// references (including recursion and forward references) resolve
	// regardless of source order.
	if err := c.load(mod, f, baseDir, reg, argv); err != nil {
		return nil, loadError(err, module)
	}

	ct, _, err := body(f)
	if err == nil && ct != ctrlNone {
		err = strayCtrl(ct)
	}
	if err != nil {
		var pos token.Pos
		if len(mod.Body) > 0 {
			pos = mod.Body[0].Position()
		}
		err, pos = takePosition(err, pos)
		return nil, object.WithFrame(err, stackFrame(module, "<module>", pos))
	}
	return g, nil
}
