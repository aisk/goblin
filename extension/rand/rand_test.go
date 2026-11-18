package rand

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aisk/goblin/object"
)

func member(name string) object.Object {
	m, _ := Execute()
	return m.(*object.Module).Members[name]
}

func call(fn object.Object, args object.Args, kws ...object.KwArg) (object.Object, error) {
	return object.Call(fn, object.CallArgs{Positional: args, Keyword: kws})
}

func mustCall(t *testing.T, fn object.Object, args object.Args, kws ...object.KwArg) object.Object {
	t.Helper()
	v, err := call(fn, args, kws...)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func method(t *testing.T, r object.Object, name string) object.Object {
	t.Helper()
	fn, err := r.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	return fn
}

func ints(n int) *object.List {
	l := &object.List{}
	for i := range n {
		l.Elements = append(l.Elements, object.Integer(i))
	}
	return l
}

func show(v object.Object) string {
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	text, _ := v.ToString()
	return text
}

func weights(ws ...object.Object) object.KwArg {
	return object.KwArg{Name: "weights", Value: &object.List{Elements: ws}}
}

// sequence draws one of everything from a generator, in a fixed order.
func sequence(t *testing.T, r object.Object) string {
	t.Helper()
	parts := []string{
		show(mustCall(t, method(t, r, "int"), object.Args{object.Integer(100)})),
		show(mustCall(t, method(t, r, "int"), object.Args{object.Integer(1), object.Integer(7)})),
		show(mustCall(t, method(t, r, "float"), nil)),
		show(mustCall(t, method(t, r, "choice"), object.Args{ints(10)})),
		show(mustCall(t, method(t, r, "choice"), object.Args{ints(3)}, weights(object.Integer(90), object.Integer(9), object.Integer(1)))),
		show(mustCall(t, method(t, r, "sample"), object.Args{ints(10), object.Integer(3)})),
		show(mustCall(t, method(t, r, "shuffle"), object.Args{ints(6)})),
	}
	return strings.Join(parts, " ")
}

// A seed fixes the sequence: these values are pinned, so a change to how a
// generator draws (which would change every seeded program's output) fails here.
func TestSeededSequenceIsPinned(t *testing.T) {
	const want = "75 2 0.604093851558642 0 0 [5, 7, 2] [1, 4, 0, 5, 3, 2]"
	r := mustCall(t, member("Rand"), object.Args{object.Integer(42)})
	if got := sequence(t, r); got != want {
		t.Fatalf("Rand(42) sequence = %q", got)
	}
	again := mustCall(t, member("Rand"), object.Args{object.Integer(42)})
	if got := sequence(t, again); got != want {
		t.Fatal("two Rand(42) differ")
	}
	if show(r) != "<rand.Rand seed=42>" || r.(*Rand).Constructor() != member("Rand") {
		t.Fatal("Rand identity")
	}
	if eq, _ := r.Equals(again); eq {
		t.Fatal("generators compare by identity")
	}
	if _, err := call(member("Rand"), nil); !errors.Is(err, object.TypeError) {
		t.Fatalf("Rand() without a seed = %v", err)
	}
	if _, err := call(member("Rand"), object.Args{object.String("x")}); !errors.Is(err, object.TypeError) {
		t.Fatalf("Rand(String) = %v", err)
	}
	if attrs := r.Attributes(); strings.Join(attrs, ",") != "int,float,choice,sample,shuffle" {
		t.Fatalf("Rand members = %v", attrs)
	}
}

func TestIntIsInRange(t *testing.T) {
	for range 200 {
		v := mustCall(t, member("int"), object.Args{object.Integer(6)}).(object.Integer)
		if v < 0 || v >= 6 {
			t.Fatalf("int(6) = %d", v)
		}
		v = mustCall(t, member("int"), object.Args{object.Integer(-3), object.Integer(-1)}).(object.Integer)
		if v != -3 && v != -2 {
			t.Fatalf("int(-3, -1) = %d", v)
		}
		v = mustCall(t, member("int"), nil, object.KwArg{Name: "end", Value: object.Integer(1)}).(object.Integer)
		if v != 0 {
			t.Fatalf("int(end=1) = %d", v)
		}
		f := mustCall(t, member("float"), nil).(object.Float)
		if f < 0 || f >= 1 {
			t.Fatalf("float() = %v", f)
		}
	}
	// The whole Int range, wider than int64 can count.
	mustCall(t, member("int"), object.Args{object.Integer(-1 << 63), object.Integer(1<<63 - 1)})
	for _, args := range []object.Args{{object.Integer(0)}, {object.Integer(5), object.Integer(5)}, {object.Integer(5), object.Integer(1)}} {
		_, err := call(member("int"), args)
		if !errors.Is(err, object.ValueError) || err.Error() != "int() range is empty" {
			t.Errorf("int%v = %v", args, err)
		}
	}
	if _, err := call(member("int"), object.Args{object.Float(2)}); !errors.Is(err, object.TypeError) {
		t.Fatalf("int(Float) = %v", err)
	}
}

func TestChoice(t *testing.T) {
	seen := map[object.Object]bool{}
	for range 300 {
		seen[mustCall(t, member("choice"), object.Args{object.String("abc")})] = true
	}
	if len(seen) != 3 {
		t.Fatalf("choice over a String saw %v", seen)
	}
	// Weights: a zero weight is never picked; the counts follow the weights.
	counts := map[object.Object]int{}
	r := mustCall(t, member("Rand"), object.Args{object.Integer(7)})
	for range 10000 {
		v := mustCall(t, method(t, r, "choice"), object.Args{ints(4)}, weights(object.Integer(90), object.Integer(0), object.Float(9.5), object.Float(0.5)))
		counts[v]++
	}
	if counts[object.Integer(1)] != 0 || counts[object.Integer(0)] < 8500 || counts[object.Integer(2)] < 700 || counts[object.Integer(3)] == 0 {
		t.Fatalf("weighted counts = %v", counts)
	}
	only := mustCall(t, member("choice"), object.Args{ints(3)}, weights(object.Integer(0), object.Integer(0), object.Integer(2)))
	if only != object.Integer(2) {
		t.Fatalf("single positive weight = %v", only)
	}
	for _, c := range []struct {
		args object.Args
		kws  []object.KwArg
		kind *object.Error
		msg  string
	}{
		{object.Args{ints(0)}, nil, object.ValueError, "choice() argument 'items' is empty"},
		{object.Args{ints(2)}, []object.KwArg{weights(object.Integer(1))}, object.ValueError, "choice() argument 'weights' must have one weight per item, got 1 weights for 2 items"},
		{object.Args{ints(2)}, []object.KwArg{weights(object.Integer(1), object.Integer(-1))}, object.ValueError, "choice() argument 'weights' must contain non-negative finite numbers, got -1"},
		{object.Args{ints(2)}, []object.KwArg{weights(object.Integer(0), object.Float(0))}, object.ValueError, "choice() argument 'weights' must not be all zero"},
		{object.Args{ints(1)}, []object.KwArg{weights(object.String("1"))}, object.TypeError, "choice() argument 'weights' must contain only Int or Float, got String"},
		{object.Args{ints(1)}, []object.KwArg{{Name: "weights", Value: object.Integer(1)}}, object.TypeError, "choice() argument 'weights' must be List or Nil, got Int"},
		{object.Args{object.Integer(3)}, nil, object.TypeError, "choice() argument 'items' must be iterable, got Int"},
	} {
		_, err := call(member("choice"), c.args, c.kws...)
		if !errors.Is(err, c.kind) || err.Error() != c.msg {
			t.Errorf("choice(%v) = %v, want %s", c.args, err, c.msg)
		}
	}
	if v := mustCall(t, member("choice"), object.Args{ints(1)}, object.KwArg{Name: "weights", Value: object.Nil}); v != object.Integer(0) {
		t.Fatalf("weights=nil = %v", v)
	}
}

func TestSampleAndShuffle(t *testing.T) {
	items := &object.List{Elements: []object.Object{object.String("a"), object.String("a"), object.String("b"), object.String("c")}}
	for range 100 {
		got := mustCall(t, member("sample"), object.Args{items, object.Integer(4)}).(*object.List)
		counts := map[object.Object]int{}
		for _, v := range got.Elements {
			counts[v]++
		}
		if counts[object.String("a")] != 2 || counts[object.String("b")] != 1 || counts[object.String("c")] != 1 {
			t.Fatalf("sample by position = %s", show(got))
		}
	}
	if items.Elements[0] != object.String("a") || items.Elements[3] != object.String("c") {
		t.Fatal("sample changed its input")
	}
	if got := mustCall(t, member("sample"), object.Args{items, object.Integer(0)}); show(got) != "[]" {
		t.Fatalf("sample 0 = %s", show(got))
	}
	for _, c := range []struct {
		count int64
		msg   string
	}{{-1, "sample() argument 'count' must be non-negative, got -1"}, {5, "sample() argument 'count' is larger than the number of items (4), got 5"}} {
		_, err := call(member("sample"), object.Args{items, object.Integer(c.count)})
		if !errors.Is(err, object.ValueError) || err.Error() != c.msg {
			t.Errorf("sample count %d = %v", c.count, err)
		}
	}
	deck := ints(52)
	got := mustCall(t, member("shuffle"), object.Args{deck})
	if got != deck {
		t.Fatal("shuffle does not return its list")
	}
	seen := map[object.Object]bool{}
	for _, v := range deck.Elements {
		seen[v] = true
	}
	if len(seen) != 52 {
		t.Fatal("shuffle lost elements")
	}
	if _, err := call(member("shuffle"), object.Args{object.String("abc")}); !errors.Is(err, object.TypeError) || err.Error() != "shuffle() argument 'list' must be List, got String" {
		t.Fatalf("shuffle(String) = %v", err)
	}
}

func TestBytes(t *testing.T) {
	a := mustCall(t, member("bytes"), object.Args{object.Integer(16)}).(object.Bytes)
	b := mustCall(t, member("bytes"), object.Args{object.Integer(16)}).(object.Bytes)
	if len(a) != 16 || string(a) == string(b) {
		t.Fatal("bytes(16)")
	}
	if got := mustCall(t, member("bytes"), object.Args{object.Integer(0)}).(object.Bytes); len(got) != 0 {
		t.Fatal("bytes(0)")
	}
	if _, err := call(member("bytes"), object.Args{object.Integer(-1)}); !errors.Is(err, object.ValueError) || err.Error() != "bytes() argument 'size' must be non-negative, got -1" {
		t.Fatalf("bytes(-1) = %v", err)
	}
	// A size beyond the limit is refused before anything is allocated.
	if _, err := call(member("bytes"), object.Args{object.Integer(1 << 62)}); !errors.Is(err, object.ValueError) || err.Error() != "bytes() argument 'size' must be at most 1073741824, got 4611686018427387904" {
		t.Fatalf("bytes(2**62) = %v", err)
	}
	saved := readSecure
	defer func() { readSecure = saved }()
	readSecure = func([]byte) (int, error) { return 0, errors.New("no entropy") }
	for _, name := range []string{"bytes", "uuid"} {
		args := object.Args{}
		if name == "bytes" {
			args = object.Args{object.Integer(4)}
		}
		if _, err := call(member(name), args); !errors.Is(err, object.IOError) {
			t.Errorf("%s with a failing source = %v", name, err)
		}
	}
}

var uuidShape = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-([47])[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUUID(t *testing.T) {
	a := string(mustCall(t, member("uuid"), nil).(object.String))
	b := string(mustCall(t, member("uuid"), nil, object.KwArg{Name: "version", Value: object.Integer(4)}).(object.String))
	if m := uuidShape.FindStringSubmatch(a); m == nil || m[1] != "4" || a == b {
		t.Fatalf("uuid() = %q, %q", a, b)
	}
	before := time.Now().UnixMilli()
	first := string(mustCall(t, member("uuid"), nil, object.KwArg{Name: "version", Value: object.Integer(7)}).(object.String))
	time.Sleep(3 * time.Millisecond)
	second := string(mustCall(t, member("uuid"), nil, object.KwArg{Name: "version", Value: object.Integer(7)}).(object.String))
	if m := uuidShape.FindStringSubmatch(first); m == nil || m[1] != "7" || !(first < second) {
		t.Fatalf("uuid(version=7) = %q, %q", first, second)
	}
	var ms int64
	fmt.Sscanf(strings.ReplaceAll(first, "-", "")[:12], "%x", &ms)
	if ms < before || ms > before+1000 {
		t.Fatalf("version 7 timestamp %d, now %d", ms, before)
	}
	for _, v := range []int64{1, 5, 0} {
		_, err := call(member("uuid"), nil, object.KwArg{Name: "version", Value: object.Integer(v)})
		if !errors.Is(err, object.ValueError) || err.Error() != fmt.Sprintf("uuid() argument 'version' must be 4 or 7, got %d", v) {
			t.Errorf("uuid(version=%d) = %v", v, err)
		}
	}
}

func TestConcurrentUse(t *testing.T) {
	r := mustCall(t, member("Rand"), object.Args{object.Integer(1)})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				if _, err := call(method(t, r, "int"), object.Args{object.Integer(10)}); err != nil {
					t.Error(err)
				}
				if _, err := call(member("choice"), object.Args{ints(5)}); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}
