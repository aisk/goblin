package object

import "sync"

// Constructed is implemented by a native value type that has a constructor:
// it returns its NativeConstructor's Function, so type_of(value) is the
// callable its module exports.
type Constructed interface {
	Constructor() *Function
}

// TypeOf returns the constructor of obj's type, what `type_of(obj)` answers.
// Every value has one: a type that cannot be constructed (Nil, Module,
// Goblin, a native handle without a constructor) answers its
// OpaqueType, so `type_of(a) == type_of(b)` and `type_of(x).name` hold for
// any value.
func TypeOf(obj Object) *Function {
	switch v := obj.(type) {
	case Integer:
		return IntConstructorFn
	case Float:
		return FloatConstructorFn
	case Bool:
		return BoolConstructorFn
	case String:
		return StringConstructorFn
	case Bytes:
		return BytesConstructorFn
	case *List:
		return ListConstructorFn
	case *Dict:
		return DictConstructorFn
	case *Error:
		return ErrorConstructorFn
	case *Function:
		return FunctionConstructorFn
	case *Chan:
		return ChanConstructorFn
	case UserValue:
		if c := v.UserType().Constructor; c != nil {
			return c
		}
	case Constructed:
		return v.Constructor()
	}
	return OpaqueType(obj.TypeName())
}

var opaqueTypes sync.Map // type name -> *Function

// OpaqueType returns the stand-in constructor of a type that cannot be
// constructed, one per type name. It is not a global; calling it raises
// TypeError.
func OpaqueType(typeName string) *Function {
	if fn, ok := opaqueTypes.Load(typeName); ok {
		return fn.(*Function)
	}
	fn := &Function{Name: typeName, Fn: func(CallArgs) (Object, error) {
		return nil, NewTypeError("%s cannot be constructed", typeName)
	}}
	actual, _ := opaqueTypes.LoadOrStore(typeName, fn)
	return actual.(*Function)
}

// Attributes performs `attributes(obj)`: a new List of the names obj's
// properties or fields, then its methods, answer to, in declaration order (a
// module's members by name).
func Attributes(obj Object) *List {
	names := obj.Attributes()
	elements := make([]Object, len(names))
	for i, name := range names {
		elements[i] = String(name)
	}
	return &List{Elements: elements}
}
