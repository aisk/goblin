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
	// Entries maps the string form of each key to its entry. Iteration order is
	// unspecified, mirroring Go's map semantics.
	Entries map[string]DictEntry
}

var _ Object = &Dict{}

func (d *Dict) Size(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("size", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("size() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	return Integer(len(d.Entries)), nil
}

func (d *Dict) Keys(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("keys", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("keys() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	keys := make([]Object, 0, len(d.Entries))
	for _, entry := range d.Entries {
		keys = append(keys, entry.Key)
	}
	return &List{Elements: keys}, nil
}

func (d *Dict) Values(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("values", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("values() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	values := make([]Object, 0, len(d.Entries))
	for _, entry := range d.Entries {
		values = append(values, entry.Value)
	}
	return &List{Elements: values}, nil
}

func NewDict() *Dict {
	return &Dict{
		Entries: make(map[string]DictEntry),
	}
}

func (d *Dict) Set(key, value Object) {
	if d.Entries == nil {
		d.Entries = make(map[string]DictEntry)
	}
	d.Entries[key.String()] = DictEntry{Key: key, Value: value}
}

func (d *Dict) Get(key Object) (Object, bool) {
	if entry, ok := d.Entries[key.String()]; ok {
		return entry.Value, true
	}
	return nil, false
}

func (d *Dict) String() string {
	elements := make([]string, 0, len(d.Entries))
	for _, entry := range d.Entries {
		elements = append(elements, fmt.Sprintf("%s: %s", entry.Key.String(), entry.Value.String()))
	}
	return fmt.Sprintf("{%s}", strings.Join(elements, ", "))
}

func (d *Dict) Bool() bool {
	return len(d.Entries) > 0
}

func (d *Dict) Compare(other Object) (int, error) {
	return 0, NewTypeError("cannot compare Dict and %T", other)
}

func (d *Dict) Add(other Object) (Object, error) {
	return nil, NewTypeError("cannot add Dict and %T", other)
}

func (d *Dict) Minus(other Object) (Object, error) {
	return nil, NewTypeError("cannot subtract from Dict")
}

func (d *Dict) Multiply(other Object) (Object, error) {
	return nil, NewTypeError("cannot multiply Dict and %T", other)
}

func (d *Dict) Divide(other Object) (Object, error) {
	return nil, NewTypeError("cannot divide Dict")
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
	keys := make([]Object, 0, len(d.Entries))
	for _, entry := range d.Entries {
		keys = append(keys, entry.Key)
	}
	return keys, nil
}

func (d *Dict) Index(index Object) (Object, error) {
	if val, ok := d.Get(index); ok {
		return val, nil
	}
	return nil, NewKeyError("key not found: %s", index.String())
}

func (d *Dict) SetIndex(index Object, value Object) error {
	d.Set(index, value)
	return nil
}

func (d *Dict) GetAttr(name string) (Object, error) {
	switch name {
	case "size":
		return &Function{Name: "size", Fn: d.Size}, nil
	case "keys":
		return &Function{Name: "keys", Fn: d.Keys}, nil
	case "values":
		return &Function{Name: "values", Fn: d.Values}, nil
	case "constructor":
		return DictConstructorFn, nil
	default:
		return nil, fmt.Errorf("Dict has no attribute '%s'", name)
	}
}

var DictConstructorFn = &Function{Name: "Dict", Fn: DictConstructor}

func DictConstructor(args CallArgs) (Object, error) {
	if len(args.Positional) != 0 {
		return nil, NewTypeError("Dict() takes no positional arguments, got %d", len(args.Positional))
	}
	result := NewDict()
	for k, v := range args.Keyword {
		result.Set(String(k), v)
	}
	return result, nil
}
