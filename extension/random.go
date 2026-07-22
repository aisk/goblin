package extension

import (
	"fmt"
	"math"
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
		"choice":      &object.Function{Name: "choice", Fn: defaultGenerator.randomChoice},
		"shuffle":     &object.Function{Name: "shuffle", Fn: defaultGenerator.randomShuffle},
		"perm":        &object.Function{Name: "perm", Fn: defaultGenerator.randomPerm},
		"sample":      &object.Function{Name: "sample", Fn: defaultGenerator.randomSample},
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
	maxObj, hasMax := ap.OptionalAny("max")
	minObj, hasMin := ap.OptionalAny("min")
	if err := ap.Finish(); err != nil {
		return nil, err
	}

	if !hasMax {
		if hasMin {
			return nil, object.NewTypeError("int() argument 'min' requires 'max'")
		}
		g.mu.Lock()
		value := g.rng.Int63()
		g.mu.Unlock()
		return object.Integer(value), nil
	}
	if _, ok := maxObj.(object.Unit); ok {
		if hasMin {
			return nil, object.NewTypeError("int() argument 'min' requires an integer 'max'")
		}
		g.mu.Lock()
		value := g.rng.Int63()
		g.mu.Unlock()
		return object.Integer(value), nil
	}

	max, ok := maxObj.(object.Integer)
	if !ok {
		return nil, object.NewTypeError("int() argument 'max' must be unit or int, got %T", maxObj)
	}
	min := object.Integer(0)
	if hasMin {
		var ok bool
		min, ok = minObj.(object.Integer)
		if !ok {
			return nil, object.NewTypeError("int() argument 'min' must be an int, got %T", minObj)
		}
	}
	if min >= max {
		return nil, object.NewValueError("int() requires min < max, got min=%d and max=%d", min, max)
	}

	width := uint64(max) - uint64(min)
	threshold := -width % width
	g.mu.Lock()
	defer g.mu.Unlock()
	for {
		r := g.rng.Uint64()
		if r >= threshold {
			return object.Integer(int64(uint64(min) + r%width)), nil
		}
	}
}

func (g *RandomGenerator) randomFloat(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("float", args)
	maxObj := ap.NumberOr("max", object.Float(1))
	minObj := ap.NumberOr("min", object.Float(0))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	max := randomNumberFloat64(maxObj)
	min := randomNumberFloat64(minObj)
	if math.IsNaN(min) || math.IsNaN(max) || math.IsInf(min, 0) || math.IsInf(max, 0) {
		return nil, object.NewValueError("float() requires finite bounds")
	}
	if min >= max {
		return nil, object.NewValueError("float() requires min < max, got min=%g and max=%g", min, max)
	}
	g.mu.Lock()
	u := g.rng.Float64()
	g.mu.Unlock()
	// A convex combination avoids overflow in max-min for wide finite ranges.
	value := min*(1-u) + max*u
	if value >= max {
		value = math.Nextafter(max, min)
	}
	return object.Float(value), nil
}

func (g *RandomGenerator) randomChoice(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("choice", args)
	listObj := ap.Any("list")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	list, ok := listObj.(*object.List)
	if !ok {
		return nil, object.NewTypeError("choice() argument 'list' must be a list, got %T", listObj)
	}
	if len(list.Elements) == 0 {
		return nil, object.NewValueError("choice() argument 'list' cannot be empty")
	}
	g.mu.Lock()
	index := g.rng.Intn(len(list.Elements))
	g.mu.Unlock()
	return list.Elements[index], nil
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

func (g *RandomGenerator) randomSample(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("sample", args)
	listObj := ap.Any("list")
	count := ap.Int("count")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	list, ok := listObj.(*object.List)
	if !ok {
		return nil, object.NewTypeError("sample() argument 'list' must be a list, got %T", listObj)
	}
	if count < 0 || uint64(count) > uint64(len(list.Elements)) {
		return nil, object.NewValueError("sample() argument 'count' must be between 0 and %d, got %d", len(list.Elements), count)
	}

	// Perform a virtual partial Fisher-Yates shuffle. The sparse swap table
	// keeps both additional memory and random draws proportional to count.
	elements := make([]object.Object, int(count))
	swaps := make(map[int]int, int(count))
	g.mu.Lock()
	for i := 0; i < int(count); i++ {
		j := i + g.rng.Intn(len(list.Elements)-i)
		selected := j
		if mapped, ok := swaps[j]; ok {
			selected = mapped
		}
		replacement := i
		if mapped, ok := swaps[i]; ok {
			replacement = mapped
		}
		swaps[j] = replacement
		elements[i] = list.Elements[selected]
	}
	g.mu.Unlock()
	return &object.List{Elements: elements}, nil
}

func (g *RandomGenerator) randomNormal(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("normal", args)
	meanObj := ap.NumberOr("mean", object.Float(0))
	stddevObj := ap.NumberOr("stddev", object.Float(1))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	mean := randomNumberFloat64(meanObj)
	stddev := randomNumberFloat64(stddevObj)
	if math.IsNaN(mean) || math.IsInf(mean, 0) || math.IsNaN(stddev) || math.IsInf(stddev, 0) {
		return nil, object.NewValueError("normal() requires finite mean and stddev")
	}
	if stddev < 0 {
		return nil, object.NewValueError("normal() argument 'stddev' must be non-negative, got %g", stddev)
	}
	if stddev == 0 {
		return object.Float(mean), nil
	}
	g.mu.Lock()
	value := mean + stddev*g.rng.NormFloat64()
	g.mu.Unlock()
	return object.Float(value), nil
}

func (g *RandomGenerator) randomExponential(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("exponential", args)
	rateObj := ap.NumberOr("rate", object.Float(1))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	rate := randomNumberFloat64(rateObj)
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 {
		return nil, object.NewValueError("exponential() argument 'rate' must be finite and positive, got %g", rate)
	}
	g.mu.Lock()
	value := g.rng.ExpFloat64() / rate
	g.mu.Unlock()
	return object.Float(value), nil
}

func randomNumberFloat64(value object.Object) float64 {
	switch number := value.(type) {
	case object.Integer:
		return float64(number)
	case object.Float:
		return float64(number)
	default:
		panic("randomNumberFloat64 called with non-number")
	}
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
	case "choice":
		return &object.Function{Name: "choice", Fn: g.randomChoice}, nil
	case "shuffle":
		return &object.Function{Name: "shuffle", Fn: g.randomShuffle}, nil
	case "perm":
		return &object.Function{Name: "perm", Fn: g.randomPerm}, nil
	case "sample":
		return &object.Function{Name: "sample", Fn: g.randomSample}, nil
	case "normal":
		return &object.Function{Name: "normal", Fn: g.randomNormal}, nil
	case "exponential":
		return &object.Function{Name: "exponential", Fn: g.randomExponential}, nil
	}
	return nil, object.NewAttributeError("Generator has no attribute '%s'", name)
}
func (g *RandomGenerator) Attributes() []string {
	return []string{"attributes", "seed", "int", "float", "choice", "shuffle", "perm", "sample", "normal", "exponential"}
}

var _ object.Object = (*RandomGenerator)(nil)

// These wrappers retain the package-private names used by focused Go tests.
func randInt(args object.CallArgs) (object.Object, error) { return defaultGenerator.randomInt(args) }
func randFloat(args object.CallArgs) (object.Object, error) {
	return defaultGenerator.randomFloat(args)
}
func randChoice(args object.CallArgs) (object.Object, error) {
	return defaultGenerator.randomChoice(args)
}
