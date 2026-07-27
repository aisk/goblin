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
	// names is the compiling Pattern's SubexpNames slice, shared because it is
	// immutable.
	names []string
}

func newMatch(source string, indices []int, names []string) *Match {
	return &Match{
		objectBase: objectBase{typeName: "Match"},
		source:     source,
		indices:    append([]int(nil), indices...),
		names:      names,
	}
}

func (m *Match) String() string            { return fmt.Sprintf("<regexp.Match %q>", m.substring(0)) }
func (m *Match) ToString() (string, error) { return m.String(), nil }

func (m *Match) participated(index int) bool {
	return index >= 0 && index*2+1 < len(m.indices) && m.indices[index*2] >= 0
}

func (m *Match) substring(index int) string {
	if !m.participated(index) {
		return ""
	}
	return m.source[m.indices[index*2]:m.indices[index*2+1]]
}

// groupIndex resolves a group key to a group number. It returns -1, nil when
// the group exists but did not participate in the match, and an IndexError when
// no such group exists at all.
func (m *Match) groupIndex(method string, key object.Object) (int, error) {
	switch value := key.(type) {
	case object.Integer:
		if value >= 0 && int(value) < len(m.indices)/2 {
			if !m.participated(int(value)) {
				return -1, nil
			}
			return int(value), nil
		}
	case object.String:
		found := false
		for i := 1; i < len(m.names); i++ {
			if m.names[i] == string(value) {
				found = true
				if m.participated(i) {
					return i, nil
				}
			}
		}
		if found {
			return -1, nil
		}
	default:
		return 0, object.NewTypeError("%s() argument 'key' must be int or str, got %s", method, key.TypeName())
	}
	return 0, object.NewIndexError("no such capture group: %s", object.Inspect(key))
}

func (m *Match) group(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("group", args)
	key := ap.AnyOr("key", object.Integer(0))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	index, err := m.groupIndex("group", key)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		return object.Nil, nil
	}
	return object.String(m.substring(index)), nil
}

// span returns the half-open [start, end) offsets of one group, or nil when the
// group did not participate.
func (m *Match) span(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("span", args)
	key := ap.AnyOr("key", object.Integer(0))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	index, err := m.groupIndex("span", key)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		return object.Nil, nil
	}
	return &object.List{Elements: []object.Object{
		object.Integer(m.indices[index*2]),
		object.Integer(m.indices[index*2+1]),
	}}, nil
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

// namedGroups maps each capture-group name to its text. Following group(), a
// name the pattern repeats resolves to its first participating group, and a
// name that participated nowhere maps to nil.
func (m *Match) namedGroups() (object.Object, error) {
	result := object.NewDict()
	filled := make(map[string]bool)
	for i := 1; i < len(m.names); i++ {
		name := m.names[i]
		if name == "" || filled[name] {
			continue
		}
		value := object.Object(object.Nil)
		if m.participated(i) {
			value = object.String(m.substring(i))
			filled[name] = true
		}
		if err := result.Set(object.String(name), value); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (m *Match) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(m), nil
	case "source":
		return object.String(m.source), nil
	case "text":
		return object.String(m.substring(0)), nil
	case "start":
		return object.Integer(m.indices[0]), nil
	case "end":
		return object.Integer(m.indices[1]), nil
	case "groups":
		return m.groups(), nil
	case "named_groups":
		return m.namedGroups()
	case "group":
		return &object.Function{Name: "group", Fn: m.group}, nil
	case "span":
		return &object.Function{Name: "span", Fn: m.span}, nil
	default:
		return nil, object.NewAttributeError("Match has no attribute '%s'", name)
	}
}

func (m *Match) Attributes() []string {
	return []string{
		"attributes", "source", "text", "start", "end",
		"groups", "named_groups", "group", "span",
	}
}

var _ object.Object = (*Match)(nil)
