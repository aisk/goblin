// Package rand is the rand module: random choices, secure bytes and ids,
// and seeded generators for sequences that repeat on every run.
package rand

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aisk/goblin/object"
)

var (
	seedCounter atomic.Int64
	// shared is the generator behind the module functions, seeded
	// differently on every run. It is not for secrets.
	shared   = newRand(time.Now().UnixNano() ^ seedCounter.Add(1))
	randType = object.NewNativeConstructor("Rand", construct)
)

// Execute builds the rand module.
func Execute() (object.Object, error) {
	return object.NewModule("rand", map[string]object.Object{
		"Rand": randType.Function,
	}, object.Funcs{
		"int":     shared.int,
		"float":   shared.float,
		"choice":  shared.choice,
		"sample":  shared.sample,
		"shuffle": shared.shuffle,
		"bytes":   secureBytes,
		"uuid":    uuid,
	}), nil
}

// Rand owns an isolated pseudo-random sequence. The lock makes its methods
// safe to call from several goblins at once.
type Rand struct {
	object.OpaqueBase
	mu   sync.Mutex
	rng  *rand.Rand
	seed int64
}

func newRand(seed int64) *Rand {
	return &Rand{OpaqueBase: object.MakeOpaqueBase("Rand"), rng: rand.New(rand.NewSource(seed)), seed: seed}
}

func construct(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("Rand", args)
	seed := ap.Int("seed")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return newRand(int64(seed)), nil
}

// below returns a number in [0, n) for n > 0, which may be beyond int64.
func (r *Rand) below(n uint64) uint64 {
	if n <= math.MaxInt64 {
		return uint64(r.rng.Int63n(int64(n)))
	}
	for {
		if v := r.rng.Uint64(); v < n {
			return v
		}
	}
}

// int is a whole number from start up to but not including end: the
// numbers range() would give for the same arguments.
func (r *Rand) int(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("int", args)
	start, end := ap.StartEnd()
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if end <= start {
		return nil, object.NewValueError("int() range is empty")
	}
	span := uint64(int64(end)) - uint64(int64(start))
	r.mu.Lock()
	offset := r.below(span)
	r.mu.Unlock()
	return object.Integer(int64(uint64(int64(start)) + offset)), nil
}

func (r *Rand) float(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("float", args); err != nil {
		return nil, err
	}
	r.mu.Lock()
	value := r.rng.Float64()
	r.mu.Unlock()
	return object.Float(value), nil
}

// weightsArg checks choice's weights against the number of items and
// returns them as float64s with their total.
func weightsArg(list *object.List, items int) ([]float64, float64, error) {
	if len(list.Elements) != items {
		return nil, 0, object.NewValueError("choice() argument 'weights' must have one weight per item, got %d weights for %d items", len(list.Elements), items)
	}
	weights := make([]float64, items)
	total := 0.0
	for i, w := range list.Elements {
		switch n := w.(type) {
		case object.Integer:
			weights[i] = float64(n)
		case object.Float:
			weights[i] = float64(n)
		default:
			return nil, 0, object.NewTypeError("choice() argument 'weights' must contain only Int or Float, got %s", w.TypeName())
		}
		if !(weights[i] >= 0) || math.IsInf(weights[i], 1) {
			text, _ := w.ToString()
			return nil, 0, object.NewValueError("choice() argument 'weights' must contain non-negative finite numbers, got %s", text)
		}
		total += weights[i]
	}
	if total == 0 {
		return nil, 0, object.NewValueError("choice() argument 'weights' must not be all zero")
	}
	if math.IsInf(total, 1) {
		return nil, 0, object.NewValueError("choice() argument 'weights' add up to more than a Float holds")
	}
	return weights, total, nil
}

// choice picks one element of items, in proportion to weights when given.
func (r *Rand) choice(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("choice", args)
	itemsArg := ap.Any("items")
	weightsList, weighted := ap.NullableList("weights")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	items, err := object.CollectArg("choice", "items", itemsArg)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, object.NewValueError("choice() argument 'items' is empty")
	}
	if !weighted {
		r.mu.Lock()
		i := r.rng.Intn(len(items))
		r.mu.Unlock()
		return items[i], nil
	}
	weights, total, err := weightsArg(weightsList, len(items))
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	target := r.rng.Float64() * total
	r.mu.Unlock()
	last := -1
	running := 0.0
	for i, w := range weights {
		if w == 0 {
			continue
		}
		last = i
		running += w
		if target < running {
			return items[i], nil
		}
	}
	// Rounding left the target at the very end: the last item with weight.
	return items[last], nil
}

// sample is count different elements of items (by position), in random
// order: a partial Fisher-Yates shuffle of the positions.
func (r *Rand) sample(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("sample", args)
	itemsArg := ap.Any("items")
	count := ap.Int("count")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	items, err := object.CollectArg("sample", "items", itemsArg)
	if err != nil {
		return nil, err
	}
	if count < 0 {
		return nil, object.NegativeArgumentError("sample", "count", int64(count))
	}
	if int64(count) > int64(len(items)) {
		return nil, object.NewValueError("sample() argument 'count' is larger than the number of items (%d), got %d", len(items), count)
	}
	positions := make([]int, len(items))
	for i := range positions {
		positions[i] = i
	}
	result := make([]object.Object, count)
	r.mu.Lock()
	for i := range result {
		j := i + r.rng.Intn(len(positions)-i)
		positions[i], positions[j] = positions[j], positions[i]
		result[i] = items[positions[i]]
	}
	r.mu.Unlock()
	return &object.List{Elements: result}, nil
}

// shuffle puts list in random order in place and returns it.
func (r *Rand) shuffle(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("shuffle", args)
	list := ap.List("list")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.rng.Shuffle(len(list.Elements), func(i, j int) {
		list.Elements[i], list.Elements[j] = list.Elements[j], list.Elements[i]
	})
	r.mu.Unlock()
	return list, nil
}

// maxSecureBytes is the most rand.bytes makes in one call (1 GiB).
const maxSecureBytes = 1 << 30

// readSecure fills b from the operating system's secure source.
var readSecure = cryptorand.Read

func secureBytes(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("bytes", args)
	size := ap.Int("size")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, object.NegativeArgumentError("bytes", "size", int64(size))
	}
	// The bytes are made up front, so a limit keeps an absurd size from
	// exhausting memory; no script needs a GiB of random data at once.
	if size > maxSecureBytes {
		return nil, object.NewValueError("bytes() argument 'size' must be at most %d, got %d", maxSecureBytes, int64(size))
	}
	b := make([]byte, size)
	if _, err := readSecure(b); err != nil {
		return nil, object.WrapNativeError(object.IOError, "bytes() failed to read random data", err)
	}
	return object.Bytes(b), nil
}

// uuid makes a version 4 (random) or version 7 (time-ordered) UUID in its
// lower-case text form.
func uuid(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("uuid", args)
	version := ap.IntOr("version", 4)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if version != 4 && version != 7 {
		return nil, object.NewValueError("uuid() argument 'version' must be 4 or 7, got %d", version)
	}
	var b [16]byte
	if _, err := readSecure(b[:]); err != nil {
		return nil, object.WrapNativeError(object.IOError, "uuid() failed to read random data", err)
	}
	if version == 7 {
		ms := uint64(time.Now().UnixMilli())
		for i := 0; i < 6; i++ {
			b[i] = byte(ms >> (40 - 8*i))
		}
	}
	b[6] = b[6]&0x0f | byte(version)<<4
	b[8] = b[8]&0x3f | 0x80
	return object.String(formatUUID(b)), nil
}

func formatUUID(b [16]byte) string {
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func (r *Rand) String() string            { return fmt.Sprintf("<rand.Rand seed=%d>", r.seed) }
func (r *Rand) ToString() (string, error) { return r.String(), nil }

// Equals is identity: two generators are never interchangeable.
func (r *Rand) Equals(other object.Object) (bool, error) {
	value, ok := other.(*Rand)
	return ok && r == value, nil
}

// Constructor satisfies object.Constructed, so type_of answers the
// constructor the module exports.
func (r *Rand) Constructor() *object.Function { return randType.Function }

func (r *Rand) GetAttr(name string) (object.Object, error) {
	var fn func(object.CallArgs) (object.Object, error)
	switch name {
	case "int":
		fn = r.int
	case "float":
		fn = r.float
	case "choice":
		fn = r.choice
	case "sample":
		fn = r.sample
	case "shuffle":
		fn = r.shuffle
	}
	if fn != nil {
		return &object.Function{Name: name, Fn: fn}, nil
	}
	return nil, object.NewAttributeError("Rand has no attribute '%s'", name)
}

func (r *Rand) Attributes() []string {
	return []string{"int", "float", "choice", "sample", "shuffle"}
}

var _ object.Object = (*Rand)(nil)
