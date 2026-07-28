package extension

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestRandIntForms(t *testing.T) {
	r := newRand(1)
	for i := 0; i < 100; i++ {
		unbounded, err := r.randomInt(object.CallArgs{})
		if err != nil {
			t.Fatal(err)
		}
		if unbounded.(object.Integer) < 0 {
			t.Fatalf("int() = %d, want non-negative", unbounded)
		}
		bounded, err := r.randomInt(object.CallArgs{Keyword: object.Kwargs{"max": object.Integer(10)}})
		if err != nil {
			t.Fatal(err)
		}
		if value := bounded.(object.Integer); value < 0 || value >= 10 {
			t.Fatalf("int(max=10) = %d, want [0, 10)", value)
		}
	}
}

func TestRandIntRejectsInvalidMaximum(t *testing.T) {
	r := newRand(1)
	for _, max := range []object.Integer{-1, 0} {
		if _, err := r.randomInt(object.CallArgs{Positional: object.Args{max}}); err == nil {
			t.Fatalf("int(%d) succeeded", max)
		}
	}
	if _, err := r.randomInt(object.CallArgs{Positional: object.Args{object.String("bad")}}); err == nil {
		t.Fatal("int() accepted a non-integer maximum")
	}
}

func TestRandSeedIsReproducible(t *testing.T) {
	a, b := newRand(42), newRand(42)
	for i := 0; i < 100; i++ {
		left, err := a.randomInt(object.CallArgs{Positional: object.Args{object.Integer(1000)}})
		if err != nil {
			t.Fatal(err)
		}
		right, err := b.randomInt(object.CallArgs{Positional: object.Args{object.Integer(1000)}})
		if err != nil {
			t.Fatal(err)
		}
		if left != right {
			t.Fatalf("same seed diverged at draw %d: %v != %v", i, left, right)
		}
	}
}

func TestRandConstructorRequiresSeed(t *testing.T) {
	obj, err := randConstructor(object.CallArgs{Keyword: object.Kwargs{"seed": object.Integer(-9)}})
	if err != nil {
		t.Fatal(err)
	}
	if obj.(*Rand).seed != -9 {
		t.Fatalf("seed = %d, want -9", obj.(*Rand).seed)
	}
	if _, err := randConstructor(object.CallArgs{}); err == nil {
		t.Fatal("Rand() accepted a missing seed")
	}
}

func TestRandFloatAndDistributions(t *testing.T) {
	r := newRand(11)
	for i := 0; i < 100; i++ {
		obj, err := r.randomFloat(object.CallArgs{})
		if err != nil {
			t.Fatal(err)
		}
		value := obj.(object.Float)
		if value < 0 || value >= 1 {
			t.Fatalf("float() = %g, want [0, 1)", value)
		}
	}
	if _, err := r.randomNormFloat(object.CallArgs{}); err != nil {
		t.Fatal(err)
	}
	exp, err := r.randomExpFloat(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	if exp.(object.Float) < 0 {
		t.Fatalf("exp_float() = %v, want non-negative", exp)
	}
}

func TestRandShuffleAndPerm(t *testing.T) {
	a, b := newRand(7), newRand(7)
	left := &object.List{Elements: object.Args{object.Integer(1), object.Integer(2), object.Integer(3), object.Integer(4)}}
	rightObj, err := left.Copy(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	right := rightObj.(*object.List)
	if _, err := a.randomShuffle(object.CallArgs{Positional: object.Args{left}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.randomShuffle(object.CallArgs{Positional: object.Args{right}}); err != nil {
		t.Fatal(err)
	}
	if !objectEquals(left, right) {
		t.Fatalf("same seed produced different shuffles: %s != %s", left, right)
	}

	permutation, err := a.randomPerm(object.CallArgs{Positional: object.Args{object.Integer(20)}})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[object.Integer]bool)
	for _, item := range permutation.(*object.List).Elements {
		seen[item.(object.Integer)] = true
	}
	if len(seen) != 20 {
		t.Fatalf("perm(20) has %d distinct values", len(seen))
	}
}

func TestRandModuleSurface(t *testing.T) {
	moduleObj, err := ExecuteRand()
	if err != nil {
		t.Fatal(err)
	}
	members := moduleObj.(*object.Module).Members
	for _, name := range []string{"Rand", "int", "float", "perm", "shuffle", "norm_float", "exp_float"} {
		if _, ok := members[name]; !ok {
			t.Errorf("rand module is missing %q", name)
		}
	}
	for _, removed := range []string{"Generator", "choice", "sample", "normal", "exponential"} {
		if _, ok := members[removed]; ok {
			t.Errorf("rand module retained non-Go API %q", removed)
		}
	}
}
