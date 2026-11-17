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
		t.Error("Iterate(Integer) succeeded")
	}
}
