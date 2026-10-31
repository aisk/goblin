package object

import (
	"math"
)

type DictEntry struct {
	Key   Object
	Value Object
}

type Dict struct {
	NoReflectedOps
	NoAssignment
	// buckets groups entries by key hash; a bucket holds more than one entry
	// only when distinct keys collide. Iteration order is unspecified,
	// mirroring Go's map semantics.
	buckets map[uint64][]DictEntry
	count   int
}

// hashKey hashes a key for bucket placement. Only Hashable types can be dict
// keys; anything else raises TypeError.
func hashKey(key Object) (uint64, error) {
	if h, ok := key.(Hashable); ok {
		return h.Hash()
	}
	return 0, NewTypeError("unhashable dict key: %s", inspect(key))
}

// keysMatch decides, within a bucket, whether a stored key and a lookup key
// are the same dict key. That is Goblin's == with one exception: a NaN key
// matches NaN, so it can be read and deleted again even though == says
// NaN != NaN.
func keysMatch(stored, lookup Object) (bool, error) {
	if a, ok := stored.(Float); ok {
		if b, ok := lookup.(Float); ok && math.IsNaN(float64(a)) && math.IsNaN(float64(b)) {
			return true, nil
		}
	}
	return Equals(stored, lookup)
}

var _ Object = &Dict{}

// Size returns the number of entries, exposed to Goblin as the read-only
// property dict.size.
func (d *Dict) Size() Integer {
	return Integer(d.count)
}

func (d *Dict) Keys(args CallArgs) (Object, error) {
	if err := RequireNoArgs("keys", args); err != nil {
		return nil, err
	}
	keys := make([]Object, 0, d.count)
	for _, entry := range d.Entries() {
		keys = append(keys, entry.Key)
	}
	return &List{Elements: keys}, nil
}

func (d *Dict) Values(args CallArgs) (Object, error) {
	if err := RequireNoArgs("values", args); err != nil {
		return nil, err
	}
	values := make([]Object, 0, d.count)
	for _, entry := range d.Entries() {
		values = append(values, entry.Value)
	}
	return &List{Elements: values}, nil
}

func (d *Dict) Items(args CallArgs) (Object, error) {
	if err := RequireNoArgs("items", args); err != nil {
		return nil, err
	}
	items := make([]Object, 0, d.count)
	for _, entry := range d.Entries() {
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
	_, ok, err := d.Get(key)
	if err != nil {
		return nil, err
	}
	return Bool(ok), nil
}

func (d *Dict) GetValue(args CallArgs) (Object, error) {
	ap := NewArgParser("get", args)
	key := ap.Any("key")
	def := ap.AnyOr("default", Nil)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	value, ok, err := d.Get(key)
	if err != nil {
		return nil, err
	}
	if ok {
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
	value, ok, err := d.Get(key)
	if err != nil {
		return nil, err
	}
	if ok {
		return value, nil
	}
	if err := d.Set(key, def); err != nil {
		return nil, err
	}
	return def, nil
}

func (d *Dict) Pop(args CallArgs) (Object, error) {
	ap := NewArgParser("pop", args)
	key := ap.Any("key")
	def, hasDefault := ap.OptionalAny("default")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	value, ok, err := d.remove(key)
	if err != nil {
		return nil, err
	}
	if ok {
		return value, nil
	}
	if hasDefault {
		return def, nil
	}
	return nil, NewKeyError("key not found: %s", inspect(key))
}

func (d *Dict) Update(args CallArgs) (Object, error) {
	ap := NewArgParser("update", args)
	other := ap.Any("other")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	source, ok := other.(*Dict)
	if !ok {
		return nil, NewTypeError("update() argument 'other' must be Dict, got %s", other.TypeName())
	}
	for _, entry := range source.Entries() {
		if err := d.Set(entry.Key, entry.Value); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (d *Dict) Clear(args CallArgs) (Object, error) {
	if err := RequireNoArgs("clear", args); err != nil {
		return nil, err
	}
	d.buckets = make(map[uint64][]DictEntry)
	d.count = 0
	return d, nil
}

func (d *Dict) Copy(args CallArgs) (Object, error) {
	if err := RequireNoArgs("copy", args); err != nil {
		return nil, err
	}
	result := NewDict()
	for _, entry := range d.Entries() {
		if err := result.Set(entry.Key, entry.Value); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func NewDict() *Dict {
	return &Dict{
		buckets: make(map[uint64][]DictEntry),
	}
}

// Len returns the number of entries.
func (d *Dict) Len() int { return d.count }

// Entries returns the entries as a fresh slice callers may keep or reorder.
// Iteration order is unspecified, mirroring Go's map semantics.
func (d *Dict) Entries() []DictEntry {
	entries := make([]DictEntry, 0, d.count)
	for _, bucket := range d.buckets {
		entries = append(entries, bucket...)
	}
	return entries
}

func (d *Dict) Set(key, value Object) error {
	hash, err := hashKey(key)
	if err != nil {
		return err
	}
	if d.buckets == nil {
		d.buckets = make(map[uint64][]DictEntry)
	}
	bucket := d.buckets[hash]
	for i, entry := range bucket {
		match, err := keysMatch(entry.Key, key)
		if err != nil {
			return err
		}
		if match {
			// The first-inserted key stays, only the value changes, so after
			// d[1.0] = x; d[1] = y the dict still shows the key as 1.0.
			bucket[i].Value = value
			return nil
		}
	}
	d.buckets[hash] = append(bucket, DictEntry{Key: key, Value: value})
	d.count++
	return nil
}

func (d *Dict) Get(key Object) (Object, bool, error) {
	hash, err := hashKey(key)
	if err != nil {
		return nil, false, err
	}
	for _, entry := range d.buckets[hash] {
		match, err := keysMatch(entry.Key, key)
		if err != nil {
			return nil, false, err
		}
		if match {
			return entry.Value, true, nil
		}
	}
	return nil, false, nil
}

// remove deletes key's entry, reporting whether one existed and its value.
func (d *Dict) remove(key Object) (Object, bool, error) {
	hash, err := hashKey(key)
	if err != nil {
		return nil, false, err
	}
	bucket := d.buckets[hash]
	for i, entry := range bucket {
		match, err := keysMatch(entry.Key, key)
		if err != nil {
			return nil, false, err
		}
		if !match {
			continue
		}
		last := len(bucket) - 1
		bucket[i] = bucket[last]
		if last == 0 {
			delete(d.buckets, hash)
		} else {
			d.buckets[hash] = bucket[:last]
		}
		d.count--
		return entry.Value, true, nil
	}
	return nil, false, nil
}

func (d *Dict) TypeName() string { return "Dict" }

func (d *Dict) String() string {
	s, _ := (&renderer{}).dict(d)
	return s
}

func (d *Dict) ToString() (string, error) { return (&renderer{strict: true}).dict(d) }

func (d *Dict) ToBool() (bool, error) { return d.count > 0, nil }

func (d *Dict) Equals(other Object) (bool, error) {
	v, ok := other.(*Dict)
	if !ok || d.count != v.count {
		return false, nil
	}
	if d == v {
		return true, nil
	}
	if err := CheckStack(); err != nil {
		return false, err
	}
	for _, entry := range d.Entries() {
		theirs, exists, err := v.Get(entry.Key)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
		eq, err := Equals(entry.Value, theirs)
		if err != nil || !eq {
			return false, err
		}
	}
	return true, nil
}

func (d *Dict) Compare(other Object) (int, error) {
	return 0, Unsupported("Dict", ErrFmtCannotCompare)
}

func (d *Dict) Add(other Object) (Object, error) {
	return nil, Unsupported("Dict", ErrFmtCannotAdd)
}

func (d *Dict) Minus(other Object) (Object, error) {
	return nil, Unsupported("Dict", ErrFmtCannotSubtract)
}

func (d *Dict) Multiply(other Object) (Object, error) {
	return nil, Unsupported("Dict", ErrFmtCannotMultiply)
}

func (d *Dict) Divide(other Object) (Object, error) {
	return nil, Unsupported("Dict", ErrFmtCannotDivide)
}

func (d *Dict) Modulo(other Object) (Object, error) {
	return nil, Unsupported("Dict", ErrFmtCannotModulo)
}

func (d *Dict) Iter() ([]Object, error) {
	keys := make([]Object, 0, d.count)
	for _, entry := range d.Entries() {
		keys = append(keys, entry.Key)
	}
	return keys, nil
}

func (d *Dict) Index(index Object) (Object, error) {
	val, ok, err := d.Get(index)
	if err != nil {
		return nil, err
	}
	if ok {
		return val, nil
	}
	return nil, NewKeyError("key not found: %s", inspect(index))
}

func (d *Dict) SetIndex(index Object, value Object) (bool, error) {
	return true, d.Set(index, value)
}

func (d *Dict) GetAttr(name string) (Object, error) {
	switch name {
	case "attributes":
		return AttributesFunction(d), nil
	case "size":
		return d.Size(), nil
	case "constructor":
		return DictConstructorFn, nil
	}
	if fn, ok := dictMethods.bind(d, name); ok {
		return fn, nil
	}
	return nil, NewAttributeError("Dict has no attribute '%s'", name)
}

func (d *Dict) Attributes() []string { return dictMethods.attributes("size") }

var dictMethods = newMethodTable(
	"keys", "values", "items", "contains", "get", "set_default", "pop", "update",
	"clear", "copy",
)

var DictConstructorFn = &Function{Name: "Dict", Fn: DictConstructor}

func DictConstructor(args CallArgs) (Object, error) {
	if len(args.Positional) != 0 {
		return nil, NewTypeError("Dict() takes no positional arguments, got %d", len(args.Positional))
	}
	result := NewDict()
	for _, kw := range args.Keyword {
		if err := result.Set(String(kw.Name), kw.Value); err != nil {
			return nil, err
		}
	}
	return result, nil
}
