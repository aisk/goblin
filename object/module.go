package object

import (
	"fmt"
	"sort"
)

type Module struct {
	inert[moduleName]
	Name string
	// Members holds the members whose value never changes once the module
	// has loaded: library functions, and the exported names a Goblin module
	// never assigns after their definition.
	Members map[string]Object
	// Vars holds the exported names the module's own code may assign. Each
	// reads the module's variable as it is when the member is read, so an
	// importer sees the module's later assignments: an export is a live
	// binding, not a copy.
	Vars map[string]func() Object
}

var _ Object = (*Module)(nil)

func (m *Module) TypeName() string { return "Module" }

func (m *Module) String() string {
	if m.Name != "" {
		return fmt.Sprintf("<module %q>", m.Name)
	}
	return "<module>"
}

func (m *Module) ToString() (string, error) { return m.String(), nil }
func (m *Module) ToBool() (bool, error)     { return true, nil }
func (m *Module) Equals(other Object) (bool, error) {
	v, ok := other.(*Module)
	return ok && m == v, nil
}

// Hash follows identity, so modules can be dict keys.
func (m *Module) Hash() (uint64, error) { return identityHash(m), nil }

func (m *Module) GetAttr(name string) (Object, error) {
	if val, ok := m.Members[name]; ok {
		return val, nil
	}
	if get, ok := m.Vars[name]; ok {
		return get(), nil
	}
	return nil, NewAttributeError("Module has no attribute '%s'", name)
}

// Attributes lists the module's members, sorted by name.
func (m *Module) Attributes() []string {
	names := make([]string, 0, len(m.Members)+len(m.Vars))
	for name := range m.Members {
		names = append(names, name)
	}
	for name := range m.Vars {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Bind adds an exported name whose value get reads afresh at every access.
func (m *Module) Bind(name string, get func() Object) {
	if m.Vars == nil {
		m.Vars = map[string]func() Object{}
	}
	m.Vars[name] = get
}

type moduleName struct{}

func (moduleName) typeName() string { return "Module" }

// Funcs maps module member names to their Go implementations.
type Funcs map[string]func(CallArgs) (Object, error)

// NewModule builds a module from plain values and from functions, each of
// which becomes a *Function named after its key. Either map may be nil.
func NewModule(name string, values map[string]Object, funcs Funcs) *Module {
	members := make(map[string]Object, len(values)+len(funcs))
	for key, value := range values {
		members[key] = value
	}
	for key, fn := range funcs {
		members[key] = &Function{Name: key, Fn: fn}
	}
	return &Module{Name: name, Members: members}
}
