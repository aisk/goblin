package object

var (
	True  = Bool(true)
	False = Bool(false)
)

type Bool bool

var _ Object = Bool(true)

func (b Bool) String() string {
	switch b {
	case true:
		return "true"
	case false:
		return "false"
	}
	panic("never happen")
}

func (b Bool) Bool() bool {
	return bool(b)
}

func (b Bool) Compare(other Object) (int, error) {
	switch v := other.(type) {
	case Bool:
		ai, bi := boolToInt(bool(b)), boolToInt(bool(v))
		if ai < bi {
			return -1, nil
		}
		if ai > bi {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, NewTypeError("cannot compare Bool and %T", other)
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (b Bool) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case String:
		return String(b.String() + string(v)), nil
	default:
		return nil, NewTypeError("cannot add Bool and %T", other)
	}
}

func (b Bool) Minus(other Object) (Object, error) {
	return nil, NewTypeError("cannot subtract from Bool")
}

func (b Bool) Multiply(other Object) (Object, error) {
	return nil, NewTypeError("cannot multiply Bool")
}

func (b Bool) Divide(other Object) (Object, error) {
	return nil, NewTypeError("cannot divide Bool")
}

func (b Bool) And(other Object) (Object, error) {
	return Bool(b.Bool() && other.Bool()), nil
}

func (b Bool) Or(other Object) (Object, error) {
	return Bool(b.Bool() || other.Bool()), nil
}

func (b Bool) Not() (Object, error) {
	return Bool(!b.Bool()), nil
}

func (b Bool) Iter() ([]Object, error) {
	return nil, NewTypeError("Bool does not support iteration")
}

func (b Bool) Index(index Object) (Object, error) {
	return nil, NewTypeError("Bool is not indexable")
}

func (b Bool) GetAttr(name string) (Object, error) {
	switch name {
	case "constructor":
		return BoolConstructorFn, nil
	default:
		return nil, NewTypeError("Bool has no attribute '%s'", name)
	}
}

var BoolConstructorFn = &Function{Name: "Bool", Fn: BoolConstructor}

func BoolConstructor(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("Bool", args); err != nil {
		return nil, err
	}
	if len(args.Positional) == 0 {
		return False, nil
	}
	if len(args.Positional) != 1 {
		return nil, NewTypeError("Bool() takes at most 1 argument, got %d", len(args.Positional))
	}
	if args.Positional[0].Bool() {
		return True, nil
	}
	return False, nil
}
