package extension

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aisk/goblin/object"
)

var (
	autoSeedCounter  atomic.Int64
	defaultGenerator = newRandomGenerator(autoSeed())
)

func autoSeed() int64 {
	return time.Now().UnixNano() ^ autoSeedCounter.Add(1)
}

func ExecuteRandom() (object.Object, error) {
	return &object.Module{Name: "random", Members: map[string]object.Object{
		"Generator":   &object.Function{Name: "Generator", Fn: randomGeneratorConstructor},
		"int":         &object.Function{Name: "int", Fn: defaultGenerator.randomInt},
		"float":       &object.Function{Name: "float", Fn: defaultGenerator.randomFloat},
		"shuffle":     &object.Function{Name: "shuffle", Fn: defaultGenerator.randomShuffle},
		"perm":        &object.Function{Name: "perm", Fn: defaultGenerator.randomPerm},
		"normal":      &object.Function{Name: "normal", Fn: defaultGenerator.randomNormal},
		"exponential": &object.Function{Name: "exponential", Fn: defaultGenerator.randomExponential},
	}}, nil
}

// RandomGenerator owns an isolated pseudo-random sequence. Its lock makes a
// generator safe to share between Goblin goroutines, although concurrent call
// ordering is intentionally not deterministic.
type RandomGenerator struct {
	mu   sync.Mutex
	rng  *rand.Rand
	seed int64
}

func newRandomGenerator(seed int64) *RandomGenerator {
	return &RandomGenerator{rng: rand.New(rand.NewSource(seed)), seed: seed}
}

func randomGeneratorConstructor(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("Generator", args)
	seedObj := ap.AnyOr("seed", object.Nil)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	seed := autoSeed()
	if _, ok := seedObj.(object.Unit); !ok {
		value, ok := seedObj.(object.Integer)
		if !ok {
			return nil, object.NewTypeError("Generator() argument 'seed' must be unit or int, got %T", seedObj)
		}
		seed = int64(value)
	}
	return newRandomGenerator(seed), nil
}

func (g *RandomGenerator) randomInt(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("int", args)
	nObj := ap.AnyOr("n", object.Nil)
	if err := ap.Finish(); err != nil {
		return nil, err
	}

	if _, ok := nObj.(object.Unit); ok {
		g.mu.Lock()
		value := g.rng.Int63()
		g.mu.Unlock()
		return object.Integer(value), nil
	}
	n, ok := nObj.(object.Integer)
	if !ok {
		return nil, object.NewTypeError("int() argument 'n' must be unit or int, got %T", nObj)
	}
	if n <= 0 {
		return nil, object.NewValueError("int() argument 'n' must be positive, got %d", n)
	}
	g.mu.Lock()
	value := g.rng.Int63n(int64(n))
	g.mu.Unlock()
	return object.Integer(value), nil
}

func (g *RandomGenerator) randomFloat(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("float", args)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	g.mu.Lock()
	value := g.rng.Float64()
	g.mu.Unlock()
	return object.Float(value), nil
}

func (g *RandomGenerator) randomShuffle(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("shuffle", args)
	listObj := ap.Any("list")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	list, ok := listObj.(*object.List)
	if !ok {
		return nil, object.NewTypeError("shuffle() argument 'list' must be a list, got %T", listObj)
	}
	g.mu.Lock()
	g.rng.Shuffle(len(list.Elements), func(i, j int) {
		list.Elements[i], list.Elements[j] = list.Elements[j], list.Elements[i]
	})
	g.mu.Unlock()
	return object.Nil, nil
}

func (g *RandomGenerator) randomPerm(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("perm", args)
	n := ap.Int("n")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if n < 0 {
		return nil, object.NewValueError("perm() argument 'n' must be non-negative, got %d", n)
	}
	maxInt := uint64(^uint(0) >> 1)
	if uint64(n) > maxInt {
		return nil, object.NewValueError("perm() argument 'n' is too large, got %d", n)
	}
	g.mu.Lock()
	values := g.rng.Perm(int(n))
	g.mu.Unlock()
	elements := make([]object.Object, len(values))
	for i, value := range values {
		elements[i] = object.Integer(value)
	}
	return &object.List{Elements: elements}, nil
}

func (g *RandomGenerator) randomNormal(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("normal", args)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	g.mu.Lock()
	value := g.rng.NormFloat64()
	g.mu.Unlock()
	return object.Float(value), nil
}

func (g *RandomGenerator) randomExponential(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("exponential", args)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	g.mu.Lock()
	value := g.rng.ExpFloat64()
	g.mu.Unlock()
	return object.Float(value), nil
}

func (g *RandomGenerator) String() string            { return fmt.Sprintf("<random.Generator seed=%d>", g.seed) }
func (g *RandomGenerator) ToString() (string, error) { return g.String(), nil }
func (g *RandomGenerator) Bool() bool                { return true }
func (g *RandomGenerator) ToBool() (bool, error)     { return true, nil }
func (g *RandomGenerator) Equals(other object.Object) bool {
	otherGenerator, ok := other.(*RandomGenerator)
	return ok && g == otherGenerator
}
func (g *RandomGenerator) Compare(object.Object) (int, error) {
	return 0, object.NewTypeError("cannot compare Generator")
}
func (g *RandomGenerator) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot add Generator")
}
func (g *RandomGenerator) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot subtract Generator")
}
func (g *RandomGenerator) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot multiply Generator")
}
func (g *RandomGenerator) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot divide Generator")
}
func (g *RandomGenerator) Not() (object.Object, error) { return object.False, nil }
func (g *RandomGenerator) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("Generator does not support iteration")
}
func (g *RandomGenerator) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Generator is not indexable")
}
func (g *RandomGenerator) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(g), nil
	case "seed":
		return object.Integer(g.seed), nil
	case "int":
		return &object.Function{Name: "int", Fn: g.randomInt}, nil
	case "float":
		return &object.Function{Name: "float", Fn: g.randomFloat}, nil
	case "shuffle":
		return &object.Function{Name: "shuffle", Fn: g.randomShuffle}, nil
	case "perm":
		return &object.Function{Name: "perm", Fn: g.randomPerm}, nil
	case "normal":
		return &object.Function{Name: "normal", Fn: g.randomNormal}, nil
	case "exponential":
		return &object.Function{Name: "exponential", Fn: g.randomExponential}, nil
	}
	return nil, object.NewAttributeError("Generator has no attribute '%s'", name)
}
func (g *RandomGenerator) Attributes() []string {
	return []string{"attributes", "seed", "int", "float", "shuffle", "perm", "normal", "exponential"}
}

var _ object.Object = (*RandomGenerator)(nil)

// These wrappers retain the package-private names used by focused Go tests.
func randInt(args object.CallArgs) (object.Object, error) { return defaultGenerator.randomInt(args) }
func randFloat(args object.CallArgs) (object.Object, error) {
	return defaultGenerator.randomFloat(args)
}
