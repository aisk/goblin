package extension

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestRandomIntCombinesInt63AndInt63n(t *testing.T) {
	for i := 0; i < 100; i++ {
		unbounded, err := randInt(object.CallArgs{})
		if err != nil {
			t.Fatal(err)
		}
		if unbounded.(object.Integer) < 0 {
			t.Fatalf("int() = %d, want non-negative", unbounded)
		}
		bounded, err := randInt(object.CallArgs{Positional: []object.Object{object.Integer(10)}})
		if err != nil {
			t.Fatal(err)
		}
		if value := bounded.(object.Integer); value < 0 || value >= 10 {
			t.Fatalf("int(10) = %d, want [0, 10)", value)
		}
	}
	for _, n := range []object.Integer{0, -1} {
		if _, err := randInt(object.CallArgs{Positional: object.Args{n}}); err == nil {
			t.Fatalf("int(%d) succeeded", n)
		}
	}
}

func TestRandomGeneratorSeedIsReproducible(t *testing.T) {
	a := newRandomGenerator(42)
	b := newRandomGenerator(42)
	for i := 0; i < 100; i++ {
		left, err := a.randomInt(object.CallArgs{Positional: []object.Object{object.Integer(1000)}})
		if err != nil {
			t.Fatal(err)
		}
		right, err := b.randomInt(object.CallArgs{Positional: []object.Object{object.Integer(1000)}})
		if err != nil {
			t.Fatal(err)
		}
		if left != right {
			t.Fatalf("same seed diverged at draw %d: %v != %v", i, left, right)
		}
	}
}

func TestRandomGeneratorShuffleAndPerm(t *testing.T) {
	a := newRandomGenerator(7)
	b := newRandomGenerator(7)
	left := &object.List{Elements: []object.Object{object.Integer(1), object.Integer(2), object.Integer(3), object.Integer(4)}}
	right, err := left.Copy(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	rightList := right.(*object.List)
	if _, err := a.randomShuffle(object.CallArgs{Positional: []object.Object{left}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.randomShuffle(object.CallArgs{Positional: []object.Object{rightList}}); err != nil {
		t.Fatal(err)
	}
	if !left.Equals(rightList) {
		t.Fatalf("same seed produced different shuffles: %s != %s", left, rightList)
	}

	permObj, err := a.randomPerm(object.CallArgs{Positional: []object.Object{object.Integer(20)}})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[int64]bool)
	for _, item := range permObj.(*object.List).Elements {
		seen[int64(item.(object.Integer))] = true
	}
	if len(seen) != 20 {
		t.Fatalf("perm(20) has %d distinct values", len(seen))
	}
}

func TestRandomGeneratorConstructor(t *testing.T) {
	obj, err := randomGeneratorConstructor(object.CallArgs{Keyword: map[string]object.Object{"seed": object.Integer(-9)}})
	if err != nil {
		t.Fatal(err)
	}
	generator := obj.(*RandomGenerator)
	if generator.seed != -9 {
		t.Fatalf("seed = %d, want -9", generator.seed)
	}
	if _, err := randomGeneratorConstructor(object.CallArgs{Keyword: map[string]object.Object{"seed": object.String("bad")}}); err == nil {
		t.Fatal("Generator accepted a non-integer seed")
	}
}

func TestRandomFloatAndDistributionsMatchRand(t *testing.T) {
	a := newRandomGenerator(99)
	b := newRandomGenerator(99)
	gotFloat, err := a.randomFloat(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	if gotFloat != object.Float(b.rng.Float64()) {
		t.Fatalf("float = %v", gotFloat)
	}
	gotNormal, err := a.randomNormal(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	if gotNormal != object.Float(b.rng.NormFloat64()) {
		t.Fatalf("normal = %v", gotNormal)
	}
	gotExponential, err := a.randomExponential(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	if gotExponential != object.Float(b.rng.ExpFloat64()) {
		t.Fatalf("exponential = %v", gotExponential)
	}
}
