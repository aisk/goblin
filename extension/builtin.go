package extension

import (
	"io"
	"os"
	"strings"

	"github.com/aisk/goblin/object"
)

var BuiltinsModule = object.NewModule("builtin", map[string]object.Object{
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
	"OverflowError":       object.OverflowError,
	"AttributeError":      object.AttributeError,
	"NameError":           object.NameError,
	"ImportError":         object.ImportError,
	"NotExistError":       object.NotExistError,
	"ExistError":          object.ExistError,
	"PermissionError":     object.PermissionError,
	"TimeoutError":        object.TimeoutError,
	"NetworkError":        object.NetworkError,
	"NotImplementedError": object.NotImplementedError,
	"RecursionError":      object.RecursionError,
	"Int":                 object.IntConstructorFn,
	"Float":               object.FloatConstructorFn,
	"String":              object.StringConstructorFn,
	"Bytes":               object.BytesConstructorFn,
	"Bool":                object.BoolConstructorFn,
	"List":                object.ListConstructorFn,
	"Dict":                object.DictConstructorFn,
	"Chan":                object.ChanConstructorFn,
	"Function":            object.FunctionConstructorFn,
	"spawn":               object.SpawnFn,
	"wait_any":            object.WaitAnyFn,
	"Eq":                  object.EqTrait,
	"Ord":                 object.OrdTrait,
	"Hashable":            object.HashableTrait,
	"Show":                object.ShowTrait,
	"Truth":               object.TruthTrait,
	"Add":                 object.AddTrait,
	"Sub":                 object.SubTrait,
	"Mul":                 object.MulTrait,
	"Div":                 object.DivTrait,
	"Mod":                 object.ModTrait,
	"Neg":                 object.NegTrait,
	"Iter":                object.IterTrait,
	"Index":               object.IndexTrait,
}, object.Funcs{
	"print":      Print,
	"eprint":     Eprint,
	"range":      Range,
	"max":        Max,
	"min":        Min,
	"type_of":    TypeOf,
	"attributes": Attributes,
	"traits":     Traits,
})

// TypeOf returns the constructor of its argument's type; see object.TypeOf.
func TypeOf(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("type_of", args)
	value := p.Any("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.TypeOf(value), nil
}

// Attributes lists the attribute names of its argument; see
// object.Attributes.
func Attributes(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("attributes", args)
	value := p.Any("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Attributes(value), nil
}

// Traits lists the traits a user type's instance implements; see
// object.Traits.
func Traits(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("traits", args)
	value := p.Any("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	traits, err := object.Traits(value)
	if err != nil {
		return nil, err
	}
	return traits, nil
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

// Range builds the list of integers from start up to, but excluding, end. A
// single positional argument is the end and the range starts at zero, as
// does one given only `end=`.
func Range(args object.CallArgs) (object.Object, error) {
	if len(args.Positional) == 1 && len(args.Keyword) == 0 {
		args.Positional = object.Args{object.Integer(0), args.Positional[0]}
	}
	p := object.NewArgParser("range", args)
	start, end := p.IntOr("start", 0), p.Int("end")
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
// range builtin and returns them as native bounds; for the one-argument
// form the caller passes Integer(0) as start. The transpiler's
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
