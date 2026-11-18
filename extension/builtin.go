package extension

import (
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/aisk/goblin/object"
)

var BuiltinsModule = object.NewModule("builtin", map[string]object.Object{
	"Error":               object.BaseError,
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
}, object.Funcs{
	"print":      Print,
	"eprint":     Eprint,
	"range":      Range,
	"max":        Max,
	"min":        Min,
	"sum":        Sum,
	"compare":    Compare,
	"hash":       Hash,
	"type_of":    TypeOf,
	"attributes": Attributes,
	"format":     Format,
	"input":      Input,
})

// Input writes prompt to stdout, with no newline, and reads one line from
// stdin, returning it without its terminator, or nil at the end of input. It
// reads through object.Stdin, so it shares its buffer with os.stdin.
func Input(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("input", args)
	prompt := p.StrOr("prompt", "")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if prompt != "" {
		if _, err := io.WriteString(os.Stdout, string(prompt)); err != nil {
			return nil, object.WrapNativeError(object.IOError, "input() failed to write the prompt", err)
		}
	}
	line, ok, err := object.ReadLine(object.Stdin)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "input() failed to read standard input", err)
	}
	if !ok {
		return object.Nil, nil
	}
	if !utf8.ValidString(line) {
		return nil, object.NewValueError("input() line is not valid UTF-8")
	}
	return object.String(line), nil
}

// Format renders a value under a format specifier given at run time; see
// object.Format. `"{x:spec}"` runs the same code.
func Format(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("format", args)
	value, spec := p.Any("value"), p.Str("spec")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Format(value, string(spec))
}

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
	p := object.NewArgParser("range", args)
	start, end := p.StartEnd()
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

// Max and Min pick the largest or smallest value by Compare, so they order
// anything `<` orders, including a user type's Ord impl. Several positional
// arguments are the candidates; a single one must be iterable and its
// elements are. key=, a function, orders the candidates by what it returns
// for each, calling it once per candidate. On a tie max answers the later
// candidate and min the earlier one, the last and first of a stable sort.
func Max(args object.CallArgs) (object.Object, error) {
	return extreme("max", args, func(c int) bool { return c >= 0 })
}

func Min(args object.CallArgs) (object.Object, error) {
	return extreme("min", args, func(c int) bool { return c < 0 })
}

// extreme returns the candidate that replaces every earlier one; replaces
// reports whether a candidate does, given Compare(candidate, best).
func extreme(name string, args object.CallArgs, replaces func(c int) bool) (object.Object, error) {
	p := object.NewArgParser(name, args)
	values := p.Rest()
	key, hasKey := p.NullableFunc("key")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	// The candidates are walked one at a time, each key computed as its
	// candidate is reached, so a key that changes the list being walked
	// follows the rule of object.Iterate.
	var it object.Iteration
	switch len(values) {
	case 0:
		return nil, object.NewTypeError("%s() requires at least 1 argument", name)
	case 1:
		var err error
		if it, err = object.IterateArg(name, "", values[0]); err != nil {
			return nil, err
		}
	default:
		it, _ = object.Iterate(&object.List{Elements: values})
	}
	defer it.Close()
	var best, bestKey object.Object
	argv := make(object.Args, 1)
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		k := v
		if hasKey {
			argv[0] = v
			if k, err = key.Call(object.CallArgs{Positional: argv}); err != nil {
				return nil, err
			}
		}
		if best == nil {
			best, bestKey = v, k
			continue
		}
		c, err := object.Compare(k, bestKey)
		if err != nil {
			return nil, err
		}
		if replaces(c) {
			best, bestKey = v, k
		}
	}
	if best == nil {
		return nil, object.NewValueError("%s() of empty sequence", name)
	}
	return best, nil
}

// Sum adds up the elements of an iterable with +, starting from the Int 0.
func Sum(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("sum", args)
	iterable := p.Any("values")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	it, err := object.IterateArg("sum", "values", iterable)
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var total object.Object = object.Integer(0)
	for {
		item, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return total, nil
		}
		if total, err = object.Add(total, item); err != nil {
			return nil, err
		}
	}
}

// Compare orders two values the way <, <=, > and >= do, as -1, 0 or 1.
func Compare(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("compare", args)
	a, b := p.Any("a"), p.Any("b")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	c, err := object.Compare(a, b)
	if err != nil {
		return nil, err
	}
	switch {
	case c < 0:
		return object.Integer(-1), nil
	case c > 0:
		return object.Integer(1), nil
	}
	return object.Integer(0), nil
}

// Hash returns the hash a dict files its argument under; see object.Hash.
func Hash(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("hash", args)
	value := p.Any("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	h, err := object.Hash(value)
	if err != nil {
		return nil, err
	}
	return object.Integer(int64(h)), nil
}
