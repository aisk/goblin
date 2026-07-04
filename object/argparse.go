package object

// ArgParser is a fluent helper for extracting and type-checking the arguments
// of a builtin function. It removes the boilerplate of manually asserting types
// and constructing TypeErrors for every parameter.
//
// The order in which the typed accessors (Int, Str, ...) are called defines the
// positional order of the parameters, so there is no need to declare a separate
// parameter-name slice. Keyword arguments take precedence over positional ones.
//
// Errors are accumulated: once any accessor fails, subsequent accessors become
// no-ops returning zero values, and the first error is reported by Finish. This
// lets callers grab every argument up front and check for errors exactly once:
//
//	p := NewArgParser("range", args)
//	start, end := p.Int("start"), p.Int("end")
//	if err := p.Finish(); err != nil {
//		return nil, err
//	}
type ArgParser struct {
	funcName string
	call     CallArgs
	pos      int
	used     map[string]bool
	err      error
}

// NewArgParser creates an ArgParser for the given function name and call arguments.
func NewArgParser(funcName string, call CallArgs) *ArgParser {
	return &ArgParser{funcName: funcName, call: call, used: make(map[string]bool)}
}

// Err returns the first error encountered, or nil. Finish should be preferred
// when the parser owns the full argument list, as it also reports extra
// arguments.
func (p *ArgParser) Err() error {
	return p.err
}

// next returns the raw value bound to name, consuming a positional slot when the
// value is not supplied by keyword. The second result is false when no value is
// available (a required argument is missing) or when the parser is already in an
// error state.
func (p *ArgParser) next(name string) (Object, bool) {
	if p.err != nil {
		return nil, false
	}
	if v, ok := p.call.Keyword[name]; ok {
		if p.used[name] {
			p.err = NewTypeError("%s() got multiple values for argument '%s'", p.funcName, name)
			return nil, false
		}
		p.used[name] = true
		return v, true
	}
	if p.pos < len(p.call.Positional) {
		v := p.call.Positional[p.pos]
		p.pos++
		return v, true
	}
	return nil, false
}

// optional returns the raw value for name if present, and whether it was found.
// Unlike next it never records a "missing argument" error.
func (p *ArgParser) optional(name string) (Object, bool) {
	return p.next(name)
}

// required fetches the raw value for name, recording a missing-argument error
// when absent.
func (p *ArgParser) required(name string) (Object, bool) {
	v, ok := p.next(name)
	if !ok && p.err == nil {
		p.err = NewTypeError("%s() missing required argument: '%s'", p.funcName, name)
	}
	return v, ok
}

// typeErr records a type mismatch error for the given argument, unless one is
// already pending.
func (p *ArgParser) typeErr(name, want string, got Object) {
	if p.err == nil {
		p.err = NewTypeError("%s() argument '%s' must be %s, got %T", p.funcName, name, want, got)
	}
}

// Any returns the raw Object bound to a required argument.
func (p *ArgParser) Any(name string) Object {
	v, ok := p.required(name)
	if !ok {
		return nil
	}
	return v
}

// AnyOr returns the raw Object bound to an optional argument, or def when absent.
func (p *ArgParser) AnyOr(name string, def Object) Object {
	v, ok := p.optional(name)
	if !ok {
		return def
	}
	return v
}

// argValue extracts a required argument, asserting it has the concrete type T
// and recording a type error (naming the expected type want) on mismatch. It
// backs the typed accessor methods below; it is a package-level function only
// because Go methods cannot take their own type parameters.
func argValue[T Object](p *ArgParser, name, want string) T {
	var zero T
	v, ok := p.required(name)
	if !ok {
		return zero
	}
	t, ok := v.(T)
	if !ok {
		p.typeErr(name, want, v)
		return zero
	}
	return t
}

// argValueOr extracts an optional argument of concrete type T, returning def
// when the argument is absent.
func argValueOr[T Object](p *ArgParser, name, want string, def T) T {
	v, ok := p.optional(name)
	if !ok {
		return def
	}
	t, ok := v.(T)
	if !ok {
		p.typeErr(name, want, v)
		return def
	}
	return t
}

// Int returns a required Integer argument.
func (p *ArgParser) Int(name string) Integer { return argValue[Integer](p, name, "int") }

// IntOr returns an optional Integer argument, or def when absent.
func (p *ArgParser) IntOr(name string, def Integer) Integer {
	return argValueOr(p, name, "int", def)
}

// Float returns a required Float argument.
func (p *ArgParser) Float(name string) Float { return argValue[Float](p, name, "float") }

// FloatOr returns an optional Float argument, or def when absent.
func (p *ArgParser) FloatOr(name string, def Float) Float {
	return argValueOr(p, name, "float", def)
}

// Str returns a required String argument.
func (p *ArgParser) Str(name string) String { return argValue[String](p, name, "str") }

// StrOr returns an optional String argument, or def when absent.
func (p *ArgParser) StrOr(name string, def String) String {
	return argValueOr(p, name, "str", def)
}

// Bool returns a required Bool argument.
func (p *ArgParser) Bool(name string) Bool { return argValue[Bool](p, name, "bool") }

// BoolOr returns an optional Bool argument, or def when absent.
func (p *ArgParser) BoolOr(name string, def Bool) Bool {
	return argValueOr(p, name, "bool", def)
}

// Func returns a required Function argument.
func (p *ArgParser) Func(name string) *Function { return argValue[*Function](p, name, "function") }

// Number returns a required argument that must be an Integer or Float, as the
// original Object so callers that need to distinguish the two kinds can
// type-switch on it. It backs the typical "numeric" parameter of mathematical
// builtins; once the parser has accepted the value no further type check is
// needed.
func (p *ArgParser) Number(name string) Object {
	v, ok := p.required(name)
	if !ok {
		return nil
	}
	switch v.(type) {
	case Integer, Float:
		return v
	default:
		p.typeErr(name, "number", v)
		return nil
	}
}

// NumberOr returns an optional Integer or Float argument, or def when absent.
func (p *ArgParser) NumberOr(name string, def Object) Object {
	v, ok := p.optional(name)
	if !ok {
		return def
	}
	switch v.(type) {
	case Integer, Float:
		return v
	default:
		p.typeErr(name, "number", v)
		return def
	}
}

// Float64 returns a required numeric (Integer or Float) argument as a float64.
// It is a convenience for mathematical builtins that do not need to distinguish
// the two kinds; callers that must (e.g. to preserve int-ness) should use
// Number instead.
func (p *ArgParser) Float64(name string) float64 {
	v := p.Number(name)
	switch n := v.(type) {
	case Integer:
		return float64(int64(n))
	case Float:
		return float64(n)
	}
	return 0
}

// Rest consumes and returns all remaining positional arguments. It should be
// called after the fixed positional accessors and captures the variadic tail.
func (p *ArgParser) Rest() Args {
	if p.err != nil {
		return nil
	}
	rest := p.call.Positional[p.pos:]
	p.pos = len(p.call.Positional)
	return rest
}

// Finish reports the first error encountered, or an error describing any
// unconsumed positional arguments or unexpected keyword arguments. Callers that
// accept an open-ended argument list should call Rest before Finish.
func (p *ArgParser) Finish() error {
	if p.err != nil {
		return p.err
	}
	if p.pos < len(p.call.Positional) {
		return NewTypeError("%s() takes %d positional arguments, got %d", p.funcName, p.pos, len(p.call.Positional))
	}
	for key := range p.call.Keyword {
		if !p.used[key] {
			return NewTypeError("%s() got an unexpected keyword argument '%s'", p.funcName, key)
		}
	}
	return nil
}
