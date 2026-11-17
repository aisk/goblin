package regex

import (
	"fmt"
	"sync"
	"unicode/utf8"

	"github.com/aisk/goblin/object"
)

// Match is an immutable snapshot of one match. It keeps Go's byte offsets
// and reports positions in characters, like every String position.
type Match struct {
	object.OpaqueBase
	source  string
	indices []int
	// names is the pattern's SubexpNames slice, shared because it is
	// immutable.
	names []string
	// ascii is set when byte offsets are already character positions.
	ascii bool

	groupsOnce sync.Once
	groupsList *object.List
}

func newMatch(source string, indices []int, names []string) *Match {
	return &Match{
		OpaqueBase: object.MakeOpaqueBase("Match"),
		source:     source,
		indices:    append([]int(nil), indices...),
		names:      names,
		ascii:      isASCII(source),
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// Constructor makes type_of(m) answer regex.Match.
func (m *Match) Constructor() *object.Function { return MatchType.Function }

func (m *Match) String() string            { return fmt.Sprintf("<regex.Match %q>", m.substring(0)) }
func (m *Match) ToString() (string, error) { return m.String(), nil }

// position converts a byte offset into the searched text to a character
// position.
func (m *Match) position(offset int) object.Integer {
	if m.ascii {
		return object.Integer(offset)
	}
	return object.Integer(utf8.RuneCountInString(m.source[:offset]))
}

func (m *Match) participated(index int) bool {
	return m.indices[index*2] >= 0
}

func (m *Match) substring(index int) string {
	return m.source[m.indices[index*2]:m.indices[index*2+1]]
}

// groupIndex resolves a group key to a group number: -1 when the group
// exists but took no part in the match, an IndexError when the pattern has
// no such group. A name used for several groups is the last one that took
// part, as to_dict() reports it. method names the caller in a TypeError, or
// is "" for indexing.
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
		found, index := false, -1
		for i := 1; i < len(m.names); i++ {
			if m.names[i] == string(value) {
				found = true
				if m.participated(i) {
					index = i
				}
			}
		}
		if found {
			return index, nil
		}
	default:
		if method == "" {
			return 0, object.NewTypeError("Match index must be Int or String, got %s", key.TypeName())
		}
		return 0, object.NewTypeError("%s() argument 'key' must be Int or String, got %s", method, key.TypeName())
	}
	return 0, object.NewIndexError("no such capture group: %s", fmt.Sprint(key))
}

// Index is m[key]: a group's text, or nil when it took no part.
func (m *Match) Index(key object.Object) (object.Object, error) {
	index, err := m.groupIndex("", key)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		return object.Nil, nil
	}
	return object.String(m.substring(index)), nil
}

// span is [start, end] of one group in characters, or nil when the group
// took no part.
func (m *Match) span(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("span", args)
	key := ap.Any("key")
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
		m.position(m.indices[index*2]),
		m.position(m.indices[index*2+1]),
	}}, nil
}

// groups is the List of groups 1…n, made once so every read is the same
// List.
func (m *Match) groups() *object.List {
	m.groupsOnce.Do(func() {
		items := make([]object.Object, len(m.indices)/2-1)
		for i := range items {
			if m.participated(i + 1) {
				items[i] = object.String(m.substring(i + 1))
			} else {
				items[i] = object.Nil
			}
		}
		m.groupsList = &object.List{Elements: items}
	})
	return m.groupsList
}

// toDict maps every named group to its text, in the order the names first
// appear in the pattern. A name used for several groups gets the last one
// that took part, nil when none did.
func (m *Match) toDict(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("to_dict", args); err != nil {
		return nil, err
	}
	result := object.NewDict()
	for i := 1; i < len(m.names); i++ {
		name := m.names[i]
		if name == "" {
			continue
		}
		key := object.String(name)
		if m.participated(i) {
			if err := result.Set(key, object.String(m.substring(i))); err != nil {
				return nil, err
			}
		} else if _, ok, _ := result.Get(key); !ok {
			if err := result.Set(key, object.Nil); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func (m *Match) GetAttr(name string) (object.Object, error) {
	switch name {
	case "text":
		return object.String(m.substring(0)), nil
	case "start":
		return m.position(m.indices[0]), nil
	case "end":
		return m.position(m.indices[1]), nil
	case "groups":
		return m.groups(), nil
	case "to_dict":
		return &object.Function{Name: "to_dict", Fn: m.toDict}, nil
	case "span":
		return &object.Function{Name: "span", Fn: m.span}, nil
	}
	return nil, object.NewAttributeError("Match has no attribute '%s'", name)
}

func (m *Match) Attributes() []string {
	return []string{"text", "start", "end", "groups", "to_dict", "span"}
}

var _ object.Object = (*Match)(nil)
