package object

import "testing"

func drain(t *testing.T, it Iteration) []Object {
	t.Helper()
	var got []Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			return got
		}
		got = append(got, v)
	}
}

// A pass over a List walks the element slice it held when the pass started,
// exactly like the Go range over List.Iter it replaces: element assignment
// shows through, while push does not lengthen the pass.
func TestIterateListKeepsRangeSemantics(t *testing.T) {
	l := &List{Elements: []Object{Integer(1), Integer(2), Integer(3)}}
	it, err := Iterate(l)
	if err != nil {
		t.Fatal(err)
	}
	var got []Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		got = append(got, v)
		if v == Integer(1) {
			l.Elements[2] = Integer(30)
			l.Elements = append(l.Elements, Integer(4))
		}
	}
	if inspect(&List{Elements: got}) != "[1, 2, 30]" {
		t.Errorf("pass = %s, want [1, 2, 30]", inspect(&List{Elements: got}))
	}
}

func TestIterateStringAndBytes(t *testing.T) {
	it, err := Iterate(String("h\xffé"))
	if err != nil {
		t.Fatal(err)
	}
	if got := inspect(&List{Elements: drain(t, it)}); got != "[\"h\", \"\uFFFD\", \"é\"]" {
		t.Errorf("String pass = %s", got)
	}
	it, err = Iterate(Bytes("ab"))
	if err != nil {
		t.Fatal(err)
	}
	if got := inspect(&List{Elements: drain(t, it)}); got != "[97, 98]" {
		t.Errorf("Bytes pass = %s", got)
	}
}

func TestCollectEmptyIsNotNil(t *testing.T) {
	for _, v := range []Object{String(""), Bytes{}} {
		items, err := Collect(v)
		if err != nil || items == nil || len(items) != 0 {
			t.Errorf("Collect(%s) = %v, %v; want an empty non-nil slice", inspect(v), items, err)
		}
	}
}

func TestIterateNotIterable(t *testing.T) {
	if _, err := Iterate(Integer(1)); err == nil {
		t.Error("Iterate(Int) succeeded")
	}
}

// countingIterable is a lazy iterable over 0 .. n-1 whose passes count how
// often they are closed. Next fails at failAt when that is not negative.
type countingIterable struct {
	List
	n, failAt int
	closes    int
}

func (c *countingIterable) Iterator() (Iterator, error) {
	i := 0
	return IteratorWithClose(func() (Object, bool, error) {
		if i == c.failAt {
			return nil, false, NewValueError("failed at %d", i)
		}
		if i >= c.n {
			return nil, false, nil
		}
		i++
		return Integer(i - 1), true, nil
	}, func() { c.closes++ }), nil
}

func TestCollectClosesThePass(t *testing.T) {
	ok := &countingIterable{n: 3, failAt: -1}
	items, err := Collect(ok)
	if err != nil || len(items) != 3 {
		t.Fatalf("Collect = %v, %v", items, err)
	}
	failing := &countingIterable{n: 3, failAt: 1}
	if _, err := Collect(failing); err == nil {
		t.Fatal("Collect of a failing pass succeeded")
	}
	if ok.closes != 1 || failing.closes != 1 {
		t.Errorf("closes = %d, %d; want 1, 1", ok.closes, failing.closes)
	}
}

func TestIterationCloseIsIdempotent(t *testing.T) {
	c := &countingIterable{n: 3, failAt: -1}
	it, err := Iterate(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := it.Next(); err != nil {
		t.Fatal(err)
	}
	it.Close()
	it.Close()
	if c.closes != 1 {
		t.Errorf("closes = %d, want 1", c.closes)
	}
	// A pass over a slice has nothing to close.
	list, _ := Iterate(&List{Elements: []Object{Integer(1)}})
	list.Close()
}

// A user type's Iter.iter result is iterated by delegation; closing the
// outer pass closes the delegated one.
func TestDelegatedPassForwardsClose(t *testing.T) {
	c := &countingIterable{n: 3, failAt: -1}
	inner, err := Iterate(c)
	if err != nil {
		t.Fatal(err)
	}
	outer := IterationOf(&inner)
	outer.Close()
	if c.closes != 1 {
		t.Errorf("closes = %d, want 1", c.closes)
	}
}
