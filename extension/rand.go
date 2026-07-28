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
	randSeedCounter atomic.Int64
	defaultRand     = newRand(autoRandSeed())
)

func autoRandSeed() int64 {
	return time.Now().UnixNano() ^ randSeedCounter.Add(1)
}

// ExecuteRand builds the rand module, modelled on Go's math/rand package.
func ExecuteRand() (object.Object, error) {
	return &object.Module{Name: "rand", Members: map[string]object.Object{
		"Rand":       &object.Function{Name: "Rand", Fn: randConstructor},
		"int":        &object.Function{Name: "int", Fn: defaultRand.randomInt},
		"float":      &object.Function{Name: "float", Fn: defaultRand.randomFloat},
		"perm":       &object.Function{Name: "perm", Fn: defaultRand.randomPerm},
		"shuffle":    &object.Function{Name: "shuffle", Fn: defaultRand.randomShuffle},
		"norm_float": &object.Function{Name: "norm_float", Fn: defaultRand.randomNormFloat},
		"exp_float":  &object.Function{Name: "exp_float", Fn: defaultRand.randomExpFloat},
	}}, nil
}

// Rand owns an isolated pseudo-random sequence. The lock makes its methods
// safe to call from concurrent Goblin code, matching Go's top-level helpers.
type Rand struct {
	object.NoReflectedOps
	object.NoAssignment
	mu   sync.Mutex
	rng  *rand.Rand
	seed int64
}

func newRand(seed int64) *Rand {
	return &Rand{rng: rand.New(rand.NewSource(seed)), seed: seed}
}

func randConstructor(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("Rand", args)
	seed := ap.Int("seed")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return newRand(int64(seed)), nil
}

// randomInt merges Go's Int63 and Int63n family. With no maximum it returns a
// non-negative int64; with max it returns a value in [0, max).
func (r *Rand) randomInt(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("int", args)
	max, bounded := ap.OptionalInt("max")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !bounded {
		return object.Integer(r.rng.Int63()), nil
	}
	if max <= 0 {
		return nil, object.NewValueError("int() argument 'max' must be positive, got %d", max)
	}
	return object.Integer(r.rng.Int63n(int64(max))), nil
}

func (r *Rand) randomFloat(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("float", args).Finish(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	value := r.rng.Float64()
	r.mu.Unlock()
	return object.Float(value), nil
}

func (r *Rand) randomPerm(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("perm", args)
	n := ap.Int("n")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if n < 0 {
		return nil, object.NewValueError("perm() argument 'n' must be non-negative, got %d", n)
	}
	if uint64(n) > uint64(^uint(0)>>1) {
		return nil, object.NewValueError("perm() argument 'n' is too large, got %d", n)
	}
	r.mu.Lock()
	values := r.rng.Perm(int(n))
	r.mu.Unlock()
	elements := make([]object.Object, len(values))
	for i, value := range values {
		elements[i] = object.Integer(value)
	}
	return &object.List{Elements: elements}, nil
}

func (r *Rand) randomShuffle(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("shuffle", args)
	listObj := ap.Any("list")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	list, ok := listObj.(*object.List)
	if !ok {
		return nil, object.NewTypeError("shuffle() argument 'list' must be a list, got %s", listObj.TypeName())
	}
	r.mu.Lock()
	r.rng.Shuffle(len(list.Elements), func(i, j int) {
		list.Elements[i], list.Elements[j] = list.Elements[j], list.Elements[i]
	})
	r.mu.Unlock()
	return object.Nil, nil
}

func (r *Rand) randomNormFloat(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("norm_float", args).Finish(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	value := r.rng.NormFloat64()
	r.mu.Unlock()
	return object.Float(value), nil
}

func (r *Rand) randomExpFloat(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("exp_float", args).Finish(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	value := r.rng.ExpFloat64()
	r.mu.Unlock()
	return object.Float(value), nil
}

func (r *Rand) TypeName() string            { return "Rand" }
func (r *Rand) String() string              { return fmt.Sprintf("<rand.Rand seed=%d>", r.seed) }
func (r *Rand) ToString() (string, error)   { return r.String(), nil }
func (r *Rand) ToBool() (bool, error)       { return true, nil }
func (r *Rand) Not() (object.Object, error) { return object.False, nil }
func (r *Rand) Equals(other object.Object) (bool, error) {
	value, ok := other.(*Rand)
	return ok && r == value, nil
}
func (r *Rand) Compare(object.Object) (int, error) {
	return 0, object.NewTypeError("cannot compare Rand")
}
func (r *Rand) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot add Rand")
}
func (r *Rand) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot subtract Rand")
}
func (r *Rand) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot multiply Rand")
}
func (r *Rand) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot divide Rand")
}
func (r *Rand) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("Rand does not support iteration")
}
func (r *Rand) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Rand is not indexable")
}

func (r *Rand) GetAttr(name string) (object.Object, error) {
	if name == "attributes" {
		return object.AttributesFunction(r), nil
	}
	methods := map[string]func(object.CallArgs) (object.Object, error){
		"int": r.randomInt, "float": r.randomFloat, "perm": r.randomPerm,
		"shuffle": r.randomShuffle, "norm_float": r.randomNormFloat,
		"exp_float": r.randomExpFloat,
	}
	if fn, ok := methods[name]; ok {
		return &object.Function{Name: name, Fn: fn}, nil
	}
	return nil, object.NewAttributeError("Rand has no attribute '%s'", name)
}

func (r *Rand) Attributes() []string {
	return []string{"attributes", "int", "float", "perm", "shuffle", "norm_float", "exp_float"}
}

var _ object.Object = (*Rand)(nil)
