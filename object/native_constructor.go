package object

// NativeConstructor gives a Go-implemented value type the same constructor
// identity convention as built-in and Goblin-defined types. The Function is
// exported by the owning module, while instances return it from their
// Constructor method (see Constructed), which is what type_of answers.
type NativeConstructor struct {
	Function *Function
}

// NewNativeConstructor creates the one stable callable identity for a native
// value type. Call this once at package scope; creating one per module load
// would make type_of(value) identity depend on import timing.
func NewNativeConstructor(name string, fn func(CallArgs) (Object, error)) *NativeConstructor {
	return &NativeConstructor{Function: &Function{Name: name, Fn: fn}}
}
