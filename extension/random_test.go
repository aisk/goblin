package extension

import (
	"math"
	"testing"

	"github.com/aisk/goblin/object"
)

func TestRandomIntForms(t *testing.T) {
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

		signed, err := randInt(object.CallArgs{Keyword: map[string]object.Object{
			"min": object.Integer(-10),
			"max": object.Integer(10),
		}})
		if err != nil {
			t.Fatal(err)
		}
		if value := signed.(object.Integer); value < -10 || value >= 10 {
			t.Fatalf("int(min=-10, max=10) = %d, want [-10, 10)", value)
		}
	}
}

func TestRandomIntWideRangeDoesNotOverflow(t *testing.T) {
	for i := 0; i < 100; i++ {
		value, err := randInt(object.CallArgs{Keyword: map[string]object.Object{
			"min": object.Integer(math.MinInt64),
			"max": object.Integer(math.MaxInt64),
		}})
		if err != nil {
			t.Fatal(err)
		}
		got := value.(object.Integer)
		if got < math.MinInt64 || got >= math.MaxInt64 {
			t.Fatalf("wide-range int = %d, outside requested range", got)
		}
	}
}

func TestRandomIntRejectsInvalidRanges(t *testing.T) {
	tests := []object.CallArgs{
		{Positional: []object.Object{object.Integer(0)}},
		{Keyword: map[string]object.Object{"min": object.Integer(2), "max": object.Integer(2)}},
		{Keyword: map[string]object.Object{"min": object.Integer(3), "max": object.Integer(2)}},
		{Keyword: map[string]object.Object{"min": object.Integer(1)}},
	}
	for _, args := range tests {
		if _, err := randInt(args); err == nil {
			t.Fatalf("randInt(%v) succeeded", args)
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
