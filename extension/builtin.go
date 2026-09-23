package extension

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aisk/goblin/object"
)

var BuiltinsModule = &object.Module{
	Name: "builtin",
	Members: map[string]object.Object{
		"print":               &object.Function{Name: "print", Fn: Print},
		"eprint":              &object.Function{Name: "eprint", Fn: Eprint},
		"spawn":               &object.Function{Name: "spawn", Fn: Spawn},
		"range":               &object.Function{Name: "range", Fn: Range},
		"max":                 &object.Function{Name: "max", Fn: Max},
		"min":                 &object.Function{Name: "min", Fn: Min},
		"Error":               object.ErrorConstructorFn,
		"TypeError":           object.TypeError,
		"ValueError":          object.ValueError,
		"LookupError":         object.LookupError,
		"ArithmeticError":     object.ArithmeticError,
		"IOError":             object.IOError,
		"ParseError":          object.ParseError,
		"IndexError":          object.IndexError,
		"KeyError":            object.KeyError,
		"ZeroDivisionError":   object.ZeroDivisionError,
		"AttributeError":      object.AttributeError,
		"NameError":           object.NameError,
		"ImportError":         object.ImportError,
		"NotExistError":       object.NotExistError,
		"ExistError":          object.ExistError,
		"PermissionError":     object.PermissionError,
		"TimeoutError":        object.TimeoutError,
		"NetworkError":        object.NetworkError,
		"NotImplementedError": object.NotImplementedError,
		"Int":                 object.IntConstructorFn,
		"Float":               object.FloatConstructorFn,
		"Str":                 object.StrConstructorFn,
		"Bytes":               object.BytesConstructorFn,
		"Bool":                object.BoolConstructorFn,
		"List":                object.ListConstructorFn,
		"Dict":                object.DictConstructorFn,
		"Chan":                object.ChanConstructorFn,
		"Goblin":              object.GoblinConstructorFn,
		"Function":            object.FunctionConstructorFn,
		"Eq":                  object.EqTrait,
		"Ord":                 object.OrdTrait,
		"Hashable":            object.HashableTrait,
		"Show":                object.ShowTrait,
		"Truth":               object.TruthTrait,
		"Num":                 object.NumTrait,
		"Iter":                object.IterTrait,
		"Index":               object.IndexTrait,
	},
}

// Print writes values to stdout, separated by spaces, ending with a newline.
// Positional-only.
func Print(args object.CallArgs) (object.Object, error) {
	return writeLine("print", os.Stdout, args)
}

// Eprint is like Print, but writes to stderr. Positional-only.
func Eprint(args object.CallArgs) (object.Object, error) {
	return writeLine("eprint", os.Stderr, args)
}

func writeLine(name string, w io.Writer, args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser(name, args)
	values := p.Rest()
	if err := p.Finish(); err != nil {
		return nil, err
	}
	var b strings.Builder
	for i, arg := range values {
		if i > 0 {
			b.WriteByte(' ')
		}
		s, err := arg.ToString()
		if err != nil {
			return nil, err
		}
		b.WriteString(s)
	}
	b.WriteByte('\n')
	if _, err := io.WriteString(w, b.String()); err != nil {
		return nil, object.WrapNativeError(object.IOError, name+"() failed to write output", err)
	}
	return object.Nil, nil
}

// spawn launches a goblin function in a new goroutine, passing any extra
// positional arguments along to it. Goroutines are fire-and-forget: the
// function's return value and error are discarded, mirroring Go's `go`
// statement. Use a Chan to communicate results back.
func Spawn(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("spawn", args)
	fn := p.Func("fn")
	// The arguments only live for this call (see object.CallArgs); the
	// goroutine runs after it returns, so it gets its own copy.
	rest := append(object.Args(nil), p.Rest()...)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	object.EnterConcurrentMode()
	go func() {
		// A panic inside a goroutine would kill the whole process regardless
		// of any recovery in main; contain it here. Errors are otherwise
		// fire-and-forget, but leaving them fully silent makes failures
		// undebuggable, so report both on stderr.
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "internal error in spawned function: %v\n", r)
			}
		}()
		if _, err := fn.Call(object.CallArgs{Positional: rest}); err != nil {
			fmt.Fprintf(os.Stderr, "uncaught error in spawned function: %v\n", err)
		}
	}()
	return object.Nil, nil
}

func Range(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("range", args)
	start, end := p.Int("start"), p.Int("end")
	if err := p.Finish(); err != nil {
		return nil, err
	}

	if int64(start) >= int64(end) {
		return &object.List{Elements: []object.Object{}}, nil
	}

	elements := make([]object.Object, int64(end)-int64(start))
	for i := int64(start); i < int64(end); i++ {
		elements[i-int64(start)] = object.Integer(i)
	}

	return &object.List{Elements: elements}, nil
}

// RangeBounds validates two range arguments through the same parser as the
// range builtin and returns them as native bounds. The transpiler's
// specialised for-range loops call this instead of materialising the list, so
// a bad argument reports exactly the error range() itself would.
func RangeBounds(start, end object.Object) (int64, int64, error) {
	p := object.NewArgParser("range", object.CallArgs{Positional: object.Args{start, end}})
	s, e := p.Int("start"), p.Int("end")
	if err := p.Finish(); err != nil {
		return 0, 0, err
	}
	return int64(s), int64(e), nil
}

// Max and Min pick among their arguments by Compare, so they order anything
// `<` orders, including a user type's Ord impl. Like Ord.max and Ord.min, on
// a tie max answers the later argument and min the earlier one.
func Max(args object.CallArgs) (object.Object, error) {
	return extreme("max", args, func(c int) bool { return c >= 0 })
}

func Min(args object.CallArgs) (object.Object, error) {
	return extreme("min", args, func(c int) bool { return c < 0 })
}

// extreme returns the argument that replaces every earlier one; replaces
// reports whether a candidate does, given Compare(candidate, best).
func extreme(name string, args object.CallArgs, replaces func(c int) bool) (object.Object, error) {
	p := object.NewArgParser(name, args)
	values := p.Rest()
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, object.NewTypeError("%s() requires at least 1 argument", name)
	}
	best := values[0]
	for _, v := range values[1:] {
		c, err := object.Compare(v, best)
		if err != nil {
			return nil, err
		}
		if replaces(c) {
			best = v
		}
	}
	return best, nil
}
