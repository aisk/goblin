package object

import (
	"fmt"
	"strings"
)

type DictEntry struct {
	Key   Object
	Value Object
}

type Dict struct {
	Entries  []DictEntry
	KeyIndex map[string]int
}

func NewDict() *Dict {
	return &Dict{
		Entries:  []DictEntry{},
		KeyIndex: make(map[string]int),
	}
}

func (d *Dict) Set(key, value Object) {
	keyStr := key.String()
	if idx, ok := d.KeyIndex[keyStr]; ok {
		d.Entries[idx].Value = value
	} else {
		d.KeyIndex[keyStr] = len(d.Entries)
		d.Entries = append(d.Entries, DictEntry{Key: key, Value: value})
	}
}

func (d *Dict) Get(key Object) (Object, bool) {
	keyStr := key.String()
	if idx, ok := d.KeyIndex[keyStr]; ok {
		return d.Entries[idx].Value, true
	}
	return nil, false
}

func (d *Dict) Repr() string {
	elements := make([]string, len(d.Entries))
	for i, entry := range d.Entries {
		elements[i] = fmt.Sprintf("%s: %s", entry.Key.Repr(), entry.Value.Repr())
	}
	return fmt.Sprintf("object.Dict({%s})", strings.Join(elements, ", "))
}

func (d *Dict) String() string {
	elements := make([]string, len(d.Entries))
	for i, entry := range d.Entries {
		elements[i] = fmt.Sprintf("%s: %s", entry.Key.String(), entry.Value.String())
	}
	return fmt.Sprintf("{%s}", strings.Join(elements, ", "))
}

func (d *Dict) Bool() bool {
	return len(d.Entries) > 0
}

func (d *Dict) Compare(other Object) (int, error) {
	return 0, fmt.Errorf("cannot compare Dict and %T", other)
}

func (d *Dict) Add(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot add Dict and %T", other)
}

func (d *Dict) Minus(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot subtract from Dict")
}

func (d *Dict) Multiply(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot multiply Dict and %T", other)
}

func (d *Dict) Divide(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot divide Dict")
}

func (d *Dict) And(other Object) (Object, error) {
	return Bool(d.Bool() && other.Bool()), nil
}

func (d *Dict) Or(other Object) (Object, error) {
	return Bool(d.Bool() || other.Bool()), nil
}

func (d *Dict) Not() (Object, error) {
	return Bool(!d.Bool()), nil
}

func (d *Dict) Iter() ([]Object, error) {
	keys := make([]Object, len(d.Entries))
	for i, entry := range d.Entries {
		keys[i] = entry.Key
	}
	return keys, nil
}

func (d *Dict) Index(index Object) (Object, error) {
	if val, ok := d.Get(index); ok {
		return val, nil
	}
	return nil, fmt.Errorf("key not found: %s", index.String())
}

func (d *Dict) GetAttr(name string) (Object, error) {
	switch name {
	case "size":
		return Integer(len(d.Entries)), nil
	case "keys":
		return &Function{Name: "keys", Fn: func(args Args, kwargs KwArgs) (Object, error) {
			keys := make([]Object, len(d.Entries))
			for i, entry := range d.Entries {
				keys[i] = entry.Key
			}
			return &List{Elements: keys}, nil
		}}, nil
	case "values":
		return &Function{Name: "values", Fn: func(args Args, kwargs KwArgs) (Object, error) {
			values := make([]Object, len(d.Entries))
			for i, entry := range d.Entries {
				values[i] = entry.Value
			}
			return &List{Elements: values}, nil
		}}, nil
	default:
		return nil, fmt.Errorf("Dict has no attribute '%s'", name)
	}
}

var _ Object = &Dict{}
