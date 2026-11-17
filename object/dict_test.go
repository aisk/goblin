package object

import (
	"fmt"
	"math/rand"
	"testing"
)

func dictKeys(d *Dict) string {
	keys, _ := d.Iter()
	return inspect(&List{Elements: keys})
}

func TestDictKeepsInsertionOrder(t *testing.T) {
	d := NewDict()
	for _, k := range []string{"c", "a", "b"} {
		d.Set(String(k), Integer(1))
	}
	if got := dictKeys(d); got != `["c", "a", "b"]` {
		t.Fatalf("keys = %s", got)
	}
	// Overwriting keeps the position; removing and writing again moves the
	// key to the end.
	d.Set(String("c"), Integer(2))
	if got := dictKeys(d); got != `["c", "a", "b"]` {
		t.Fatalf("keys after overwrite = %s", got)
	}
	d.remove(String("c"))
	d.Set(String("c"), Integer(3))
	if got := dictKeys(d); got != `["a", "b", "c"]` {
		t.Fatalf("keys after remove and set = %s", got)
	}
	if got := inspect(d); got != `{"a": 1, "b": 1, "c": 3}` {
		t.Fatalf("dict = %s", got)
	}
	if got := callMethod(t, d, "remove", CallArgs{Positional: Args{String("missing")}}); got != d || d.Len() != 3 {
		t.Fatalf("remove of a missing key = %v", got)
	}
}

// collidingKey hashes every value to one bucket, so the chain handling of
// lookups and removals is exercised.
type collidingKey struct {
	inert[functionName]
	NoAssignment
	n int
}

func (k *collidingKey) Hash() (uint64, error) { return 7, nil }
func (k *collidingKey) Equals(o Object) (bool, error) {
	c, ok := o.(*collidingKey)
	return ok && c.n == k.n, nil
}
func (k *collidingKey) String() string                 { return fmt.Sprint(k.n) }
func (k *collidingKey) ToString() (string, error)      { return k.String(), nil }
func (k *collidingKey) ToBool() (bool, error)          { return true, nil }
func (k *collidingKey) TypeName() string               { return "K" }
func (k *collidingKey) GetAttr(string) (Object, error) { return nil, NewAttributeError("no") }
func (k *collidingKey) Attributes() []string           { return nil }

// TestDictMatchesAMap drives a Dict and a reference (a Go map plus an order
// slice) through random operations, colliding keys included.
func TestDictMatchesAMap(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	d := NewDict()
	ref := map[int]int{}
	var order []int
	key := func(n int) Object {
		if n%3 == 0 {
			return &collidingKey{n: n}
		}
		return Integer(n)
	}
	for step := 0; step < 20000; step++ {
		n := rng.Intn(200)
		switch rng.Intn(3) {
		case 0, 1:
			if _, ok := ref[n]; !ok {
				order = append(order, n)
			}
			ref[n] = step
			if err := d.Set(key(n), Integer(step)); err != nil {
				t.Fatal(err)
			}
		case 2:
			_, want := ref[n]
			_, got, err := d.remove(key(n))
			if err != nil || got != want {
				t.Fatalf("step %d: remove(%d) = %v, %v; want %v", step, n, got, err, want)
			}
			if want {
				delete(ref, n)
				for i, k := range order {
					if k == n {
						order = append(order[:i], order[i+1:]...)
						break
					}
				}
			}
		}
		if d.Len() != len(ref) {
			t.Fatalf("step %d: Len = %d, want %d", step, d.Len(), len(ref))
		}
	}
	entries := d.Entries()
	if len(entries) != len(order) {
		t.Fatalf("%d entries, want %d", len(entries), len(order))
	}
	for i, n := range order {
		if inspect(entries[i].Key) != fmt.Sprint(n) || entries[i].Value != Integer(ref[n]) {
			t.Fatalf("entry %d = %v: %v, want %d: %d", i, entries[i].Key, entries[i].Value, n, ref[n])
		}
		if v, ok, _ := d.Get(key(n)); !ok || v != Integer(ref[n]) {
			t.Fatalf("Get(%d) = %v, %v", n, v, ok)
		}
	}
	// Holes are compacted away, so the storage stays proportional to the size.
	if len(d.slots) > 2*d.Len()+9 {
		t.Fatalf("%d slots for %d entries", len(d.slots), d.Len())
	}
}
