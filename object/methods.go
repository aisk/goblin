package object

// methodTable is the method list of a built-in type that implements
// MethodCaller. CallMethod's switch is the one place that maps a method name to
// Go code; GetAttr binds a method by routing it back through CallMethod, and
// Attributes is built from the same list, so a method is named in exactly two
// places, which TestCallMethodMatchesGetAttr keeps in step.
type methodTable struct {
	names []string
	set   map[string]struct{}
}

func newMethodTable(names ...string) methodTable {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	return methodTable{names: names, set: set}
}

// bind returns recv's method name as a bound *Function, for `x.m` taken as a
// value rather than called on the spot.
func (t methodTable) bind(recv MethodCaller, name string) (*Function, bool) {
	if _, ok := t.set[name]; !ok {
		return nil, false
	}
	return &Function{Name: name, Fn: func(args CallArgs) (Object, error) {
		v, _, err := recv.CallMethod(name, args)
		return v, err
	}}, true
}

// attributes lists "attributes", the type's properties, its methods and
// "constructor", in that order.
func (t methodTable) attributes(properties ...string) []string {
	names := make([]string, 0, len(t.names)+len(properties)+2)
	names = append(names, "attributes")
	names = append(names, properties...)
	names = append(names, t.names...)
	return append(names, "constructor")
}
