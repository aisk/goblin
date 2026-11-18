package object

import (
	"math"
)

type DictEntry struct {
	Key   Object
	Value Object
}

// Dict keeps its entries in insertion order. slots holds them in that order,
// with a removed entry left behind as a hole (a nil Key) so the positions of
// the others stay valid; index maps a key hash to the position of the latest
// slot with that hash, and each slot links to the previous one with the same
// hash, so distinct keys that collide form a short chain. get, set and remove
// are O(1) on average; once holes make up half the slots, the next removal
// compacts them away, which keeps that amortised and iteration O(size).
type Dict struct {
	NoReflectedOps
	NoAssignment
	slots []dictSlot
	index map[uint64]int32
	count int
}

type dictSlot struct {
	DictEntry
	hash uint64
	// next is the position of the previous slot with the same hash, or -1.
	next int32
}

// hashKey hashes a key for bucket placement. Only Hashable types can be dict
// keys; anything else raises the TypeError hash() raises.
func hashKey(key Object) (uint64, error) { return Hash(key) }

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

// GetOrSet returns key's value; when there is none it stores def under key
// first.
func (d *Dict) GetOrSet(args CallArgs) (Object, error) {
	ap := NewArgParser("get_or_set", args)
	key := ap.Any("key")
	def := ap.Any("default")
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
	return nil, keyNotFound(key)
}

// Remove deletes key's entry; without one it does nothing.
func (d *Dict) Remove(args CallArgs) (Object, error) {
	ap := NewArgParser("remove", args)
	key := ap.Any("key")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if _, _, err := d.remove(key); err != nil {
		return nil, err
	}
	return d, nil
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
	d.slots, d.index, d.count = nil, nil, 0
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
	return &Dict{}
}

// Len returns the number of entries.
func (d *Dict) Len() int { return d.count }

// Entries returns the entries in insertion order, as a fresh slice callers
// may keep or reorder.
func (d *Dict) Entries() []DictEntry {
	entries := make([]DictEntry, 0, d.count)
	for i := range d.slots {
		if d.slots[i].Key != nil {
			entries = append(entries, d.slots[i].DictEntry)
		}
	}
	return entries
}

// lookup returns the position of key's slot, or -1, and key's hash.
func (d *Dict) lookup(key Object) (int32, uint64, error) {
	hash, err := hashKey(key)
	if err != nil {
		return -1, 0, err
	}
	pos, ok := d.index[hash]
	if !ok {
		return -1, hash, nil
	}
	for ; pos >= 0; pos = d.slots[pos].next {
		match, err := keysMatch(d.slots[pos].Key, key)
		if err != nil {
			return -1, hash, err
		}
		if match {
			return pos, hash, nil
		}
	}
	return -1, hash, nil
}

// Set stores value under key. A key already present keeps its position, and
// the first-inserted key object stays: after d[1.0] = x; d[1] = y the dict
// still shows the key as 1.0.
func (d *Dict) Set(key, value Object) error {
	pos, hash, err := d.lookup(key)
	if err != nil {
		return err
	}
	if pos >= 0 {
		d.slots[pos].Value = value
		return nil
	}
	if d.index == nil {
		d.index = make(map[uint64]int32)
	}
	next, ok := d.index[hash]
	if !ok {
		next = -1
	}
	d.index[hash] = int32(len(d.slots))
	d.slots = append(d.slots, dictSlot{DictEntry: DictEntry{Key: key, Value: value}, hash: hash, next: next})
	d.count++
	return nil
}

func (d *Dict) Get(key Object) (Object, bool, error) {
	pos, _, err := d.lookup(key)
	if err != nil || pos < 0 {
		return nil, false, err
	}
	return d.slots[pos].Value, true, nil
}

// remove deletes key's entry, reporting whether one existed and its value.
func (d *Dict) remove(key Object) (Object, bool, error) {
	pos, hash, err := d.lookup(key)
	if err != nil || pos < 0 {
		return nil, false, err
	}
	// Unlink the slot from its hash chain.
	if head := d.index[hash]; head == pos {
		if next := d.slots[pos].next; next >= 0 {
			d.index[hash] = next
		} else {
			delete(d.index, hash)
		}
	} else {
		for p := head; ; p = d.slots[p].next {
			if d.slots[p].next == pos {
				d.slots[p].next = d.slots[pos].next
				break
			}
		}
	}
	value := d.slots[pos].Value
	d.slots[pos] = dictSlot{next: -1}
	d.count--
	if d.count == 0 {
		d.slots, d.index = d.slots[:0], nil
	} else if holes := len(d.slots) - d.count; holes > 8 && holes > d.count {
		d.compact()
	}
	return value, true, nil
}

// compact drops the holes removals left, rebuilding the hash chains for the
// new positions. Live slots keep their relative order.
func (d *Dict) compact() {
	slots := make([]dictSlot, 0, d.count)
	index := make(map[uint64]int32, d.count)
	for _, slot := range d.slots {
		if slot.Key == nil {
			continue
		}
		next, ok := index[slot.hash]
		if !ok {
			next = -1
		}
		slot.next = next
		index[slot.hash] = int32(len(slots))
		slots = append(slots, slot)
	}
	d.slots, d.index = slots, index
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
	for i := range d.slots {
		if d.slots[i].Key != nil {
			keys = append(keys, d.slots[i].Key)
		}
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
	return nil, keyNotFound(index)
}

func (d *Dict) SetIndex(index Object, value Object) (bool, error) {
	return true, d.Set(index, value)
}

func (d *Dict) GetAttr(name string) (Object, error) {
	switch name {
	case "size":
		return d.Size(), nil
	}
	if fn, ok := dictMethods.bind(d, name); ok {
		return fn, nil
	}
	return nil, NewAttributeError("Dict has no attribute '%s'", name)
}

func (d *Dict) Attributes() []string { return dictMethods.attributes("size") }

var dictMethods = newMethodTable(
	"keys", "values", "items", "contains", "get", "get_or_set", "pop", "remove",
	"update", "clear", "copy",
)

var DictConstructorFn = &Function{Name: "Dict", Fn: DictConstructor}

func DictConstructor(args CallArgs) (Object, error) {
	if len(args.Positional) != 0 {
		return nil, NewTypeError("Dict() takes no positional arguments, got %d", len(args.Positional))
	}
	if name, dup := args.Keyword.Duplicate(); dup {
		return nil, NewTypeError("Dict() got multiple values for argument '%s'", name)
	}
	result := NewDict()
	for _, kw := range args.Keyword {
		if err := result.Set(String(kw.Name), kw.Value); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// keyNotFound is the KeyError for a missing key, which shows the key as a
// collection shows an element, so `d[1]` and `d["1"]` are told apart.
func keyNotFound(key Object) *Error {
	s, err := (&renderer{}).element(key)
	if err != nil {
		s = inspect(key)
	}
	return NewKeyError("key not found: %s", s)
}
