package object

// NativeConstructor gives a Go-implemented value type the same constructor
// identity convention as built-in and Goblin-defined types. The Function is
// exported by the owning module, while instances return it from their
// constructor attribute.
type NativeConstructor struct {
	Function *Function
}

// NewNativeConstructor creates the one stable callable identity for a native
// value type. Call this once at package scope; creating one per module load
// would make instance.constructor identity depend on import timing.
func NewNativeConstructor(name string, fn func(CallArgs) (Object, error)) *NativeConstructor {
	return &NativeConstructor{Function: &Function{Name: name, Fn: fn}}
}

// Attribute handles the constructor attribute shared by native value types.
func (c *NativeConstructor) Attribute(name string) (Object, bool) {
	if name == "constructor" {
		return c.Function, true
	}
	return nil, false
}

// Attributes returns attributes with constructor appended exactly once.
func (c *NativeConstructor) Attributes(attributes ...string) []string {
	result := append([]string(nil), attributes...)
	for _, name := range result {
		if name == "constructor" {
			return result
		}
	}
	return append(result, "constructor")
}
