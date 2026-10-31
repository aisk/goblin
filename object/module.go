package object

import (
	"fmt"
	"sort"
)

type Module struct {
	inert[moduleName]
	Name    string
	Members map[string]Object
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
	if name == "attributes" {
		return AttributesFunction(m), nil
	}
	return nil, NewAttributeError("Module has no attribute '%s'", name)
}

func (m *Module) Attributes() []string {
	names := make([]string, 0, len(m.Members)+1)
	for name := range m.Members {
		names = append(names, name)
	}
	if _, overridden := m.Members["attributes"]; !overridden {
		names = append(names, "attributes")
	}
	sort.Strings(names)
	return names
}

type moduleName struct{}

func (moduleName) typeName() string { return "Module" }
