package object

import (
	"hash/maphash"
	"math"
)

// Hashable marks a value usable as a dict key. The one law is consistency
// with Equals: values that compare equal under == must return the same hash.
// The reverse never has to hold — the dict resolves equal hashes with Equals
// (see keysMatch) — so a hash needs to be stable, not unique. Mutable
// built-ins (List, Dict, Bytes) stay non-Hashable on purpose: mutating a
// stored key would strand its entry under a stale hash.
type Hashable interface {
	Hash() (uint64, error)
}

// hashSeed keys the string hash. Hashes never leave the process, so a
// per-process random seed is fine.
var hashSeed = maphash.MakeSeed()

// Arbitrary constants keeping the singleton values' hashes apart from the
// small numbers'. Collisions would still be correct, just pointless.
const (
	hashFalse uint64 = 0x7c9a2e4b1d0f8a55
	hashTrue  uint64 = 0x3b8f1c6d5a2e9b71
	hashNil   uint64 = 0xe1d4b7a90c3f6288
)

func (v String) Hash() (uint64, error) { return maphash.String(hashSeed, string(v)), nil }

// Integer and Float hash through the float64 domain because their equality is
// decided there (see numericEquals): 1 and 1.0 are one dict key. Distinct
// huge integers may round to the same float64 and thus share a hash; that is
// a collision the dict resolves, not a merge.
func (v Integer) Hash() (uint64, error) { return hashNumber(float64(v)), nil }

func (v Float) Hash() (uint64, error) { return hashNumber(float64(v)), nil }

func (v Bool) Hash() (uint64, error) {
	if v {
		return hashTrue, nil
	}
	return hashFalse, nil
}

func (Unit) Hash() (uint64, error) { return hashNil, nil }

// hashNumber hashes a number by its float64 bits. -0.0 folds into +0.0 (they
// compare equal), and every NaN takes one canonical pattern because the dict
// treats all NaNs as a single key (see keysMatch). No mixing is needed: the
// buckets map re-hashes this value itself.
func hashNumber(f float64) uint64 {
	if f == 0 {
		f = 0
	} else if math.IsNaN(f) {
		f = math.NaN()
	}
	return math.Float64bits(f)
}
