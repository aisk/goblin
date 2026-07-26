package object

import "reflect"

// Named is implemented by objects that know their Goblin-level type name.
// User-defined types implement it in both backends so a value's type is
// reported by its declared name rather than by the Go type the backend
// happens to represent it with.
type Named interface {
	TypeName() string
}

// TypeName returns the Goblin-level name of an object's type, as used in
// diagnostics. Runtime messages must never expose Go type names: those differ
// between the interpreter and transpiled programs (`*interpreter.instance` vs
// `*main.Point`), which would make the two backends disagree on the text of
// an error.
//
// Types outside this package are named by their Go type name with any pointer
// and package qualifier stripped, which is already the Goblin-visible name for
// the standard library's types (`Time`, `Pattern`, `URL`); anything needing a
// different name implements Named.
func TypeName(obj Object) string {
	switch obj.(type) {
	case Integer:
		return "Integer"
	case Float:
		return "Float"
	case String:
		return "String"
	case Bool:
		return "Bool"
	case Bytes:
		return "Bytes"
	case Unit:
		return "Nil"
	case *List:
		return "List"
	case *Dict:
		return "Dict"
	case *Path:
		return "Path"
	case *Error:
		return "Error"
	case *Function:
		return "Function"
	case *Chan:
		return "Chan"
	case *Module:
		return "Module"
	}
	if named, ok := obj.(Named); ok {
		return named.TypeName()
	}
	t := reflect.TypeOf(obj)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil {
		return "Nil"
	}
	return t.Name()
}
