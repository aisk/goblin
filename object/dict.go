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

func (d *Dict) Items(args CallArgs) (Object, error) {
	if err := requireNoArgs("items", args); err != nil {
		return nil, err
	}
	items := make([]Object, 0, len(d.Entries))
	for _, entry := range d.Entries {
		items = append(items, &List{Elements: []Object{entry.Key, entry.Value}})
	}
	return &List{Elements: items}, nil
}

func (d *Dict) Contains(args CallArgs) (Object, error) {
	ap := NewArgParser("contains", args)
	key := ap.Any("key")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	_, ok := d.Get(key)
	return Bool(ok), nil
}

func (d *Dict) GetValue(args CallArgs) (Object, error) {
	ap := NewArgParser("get", args)
	key := ap.Any("key")
	def := ap.AnyOr("default", Nil)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if value, ok := d.Get(key); ok {
		return value, nil
	}
	return def, nil
}

func (d *Dict) SetDefault(args CallArgs) (Object, error) {
	ap := NewArgParser("set_default", args)
	key := ap.Any("key")
	def := ap.AnyOr("default", Nil)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if value, ok := d.Get(key); ok {
		return value, nil
	}
	d.Set(key, def)
	return def, nil
}

func (d *Dict) Pop(args CallArgs) (Object, error) {
	ap := NewArgParser("pop", args)
	key := ap.Any("key")
	def, hasDefault := ap.OptionalAny("default")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	encoded := key.String()
	if entry, ok := d.Entries[encoded]; ok {
		delete(d.Entries, encoded)
		return entry.Value, nil
	}
	if hasDefault {
		return def, nil
	}
	return nil, NewKeyError("key not found: %s", key.String())
}

func (d *Dict) Update(args CallArgs) (Object, error) {
	ap := NewArgParser("update", args)
	other := ap.Any("other")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	source, ok := other.(*Dict)
	if !ok {
		return nil, NewTypeError("update() argument 'other' must be Dict, got %T", other)
	}
	for _, entry := range source.Entries {
		d.Set(entry.Key, entry.Value)
	}
	return d, nil
}

func (d *Dict) Clear(args CallArgs) (Object, error) {
	if err := requireNoArgs("clear", args); err != nil {
		return nil, err
	}
	d.Entries = make(map[string]DictEntry)
	return d, nil
}

func (d *Dict) Copy(args CallArgs) (Object, error) {
	if err := requireNoArgs("copy", args); err != nil {
		return nil, err
	}
	result := NewDict()
	for _, entry := range d.Entries {
		result.Set(entry.Key, entry.Value)
	}
	return result, nil
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
	case "attributes":
		return AttributesFunction(d), nil
	case "size":
		return &Function{Name: "size", Fn: d.Size}, nil
	case "keys":
		return &Function{Name: "keys", Fn: d.Keys}, nil
	case "values":
		return &Function{Name: "values", Fn: d.Values}, nil
	case "items":
		return &Function{Name: "items", Fn: d.Items}, nil
	case "contains":
		return &Function{Name: "contains", Fn: d.Contains}, nil
	case "get":
		return &Function{Name: "get", Fn: d.GetValue}, nil
	case "set_default":
		return &Function{Name: "set_default", Fn: d.SetDefault}, nil
	case "pop":
		return &Function{Name: "pop", Fn: d.Pop}, nil
	case "update":
		return &Function{Name: "update", Fn: d.Update}, nil
	case "clear":
		return &Function{Name: "clear", Fn: d.Clear}, nil
	case "copy":
		return &Function{Name: "copy", Fn: d.Copy}, nil
	case "constructor":
		return DictConstructorFn, nil
	default:
		return nil, NewAttributeError("Dict has no attribute '%s'", name)
	}
}

func (d *Dict) Attributes() []string {
	return []string{"attributes", "size", "keys", "values", "items", "contains", "get", "set_default", "pop", "update", "clear", "copy", "constructor"}
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
