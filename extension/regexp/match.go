package regexp

import (
	"fmt"

	"github.com/aisk/goblin/object"
)

// Match is an immutable snapshot of one match. Offsets are UTF-8 byte offsets.
type Match struct {
	objectBase
	source  string
	indices []int
	names   []string
}

func newMatch(source string, indices []int, names []string) *Match {
	return &Match{source: source, indices: append([]int(nil), indices...), names: append([]string(nil), names...)}
}

func (m *Match) String() string                  { return fmt.Sprintf("<regexp.Match %q>", m.substring(0)) }
func (m *Match) ToString() (string, error)       { return m.String(), nil }
func (m *Match) Bool() bool                      { return true }
func (m *Match) ToBool() (bool, error)           { return true, nil }
func (m *Match) Not() (object.Object, error)     { return object.False, nil }
func (m *Match) Equals(other object.Object) bool { return m == other }

func (m *Match) participated(index int) bool {
	return index >= 0 && index*2+1 < len(m.indices) && m.indices[index*2] >= 0
}

func (m *Match) substring(index int) string {
	if !m.participated(index) {
		return ""
	}
	return m.source[m.indices[index*2]:m.indices[index*2+1]]
}

func (m *Match) group(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("group", args)
	key := ap.AnyOr("index_or_name", object.Integer(0))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	index := -1
	switch value := key.(type) {
	case object.Integer:
		if value >= 0 && int(value)*2+1 < len(m.indices) {
			index = int(value)
		}
	case object.String:
		found := false
		for i := 1; i < len(m.names); i++ {
			if m.names[i] == string(value) {
				found = true
				if m.participated(i) {
					index = i
					break
				}
			}
		}
		if found && index < 0 {
			return object.Nil, nil
		}
	default:
		return nil, object.NewTypeError("group() argument 'index_or_name' must be int or str, got %T", key)
	}
	if index < 0 {
		return nil, object.NewIndexError("no such capture group: %s", key.String())
	}
	if !m.participated(index) {
		return object.Nil, nil
	}
	return object.String(m.substring(index)), nil
}

func (m *Match) groups() *object.List {
	items := make([]object.Object, len(m.indices)/2-1)
	for i := range items {
		if m.participated(i + 1) {
			items[i] = object.String(m.substring(i + 1))
		} else {
			items[i] = object.Nil
		}
	}
	return &object.List{Elements: items}
}

func (m *Match) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(m), nil
	case "text":
		return object.String(m.substring(0)), nil
	case "start":
		return object.Integer(m.indices[0]), nil
	case "end":
		return object.Integer(m.indices[1]), nil
	case "groups":
		return m.groups(), nil
	case "group":
		return &object.Function{Name: "group", Fn: m.group}, nil
	default:
		return nil, object.NewAttributeError("Match has no attribute '%s'", name)
	}
}

func (m *Match) Attributes() []string {
	return []string{"attributes", "text", "start", "end", "groups", "group"}
}

var _ object.Object = (*Match)(nil)
