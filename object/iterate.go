package object

// Iteration has two Go-level shapes. Object.Iter is the eager one: it returns
// every element at once, and a type can keep implementing only that. A type
// that can hand out its elements one at a time implements Iterable as well,
// and Iterate prefers it. Everything that walks an iterable — for loops in
// both backends, destructuring, `*` arguments, List(x) — goes through Iterate
// or Collect, never through Iter directly, so a type migrates by adding
// Iterable without touching any consumer.
//
// The iterator is pulled rather than pushed (a Go iter.Seq): a for loop in a
// transpiled program then stays an ordinary Go loop whose locals the body
// updates in place, instead of a range-over-func closure that captures them.
// The price is that a consumer may stop pulling at any point (break, return,
// an error in the body) without telling the iterator, so an iterator must be
// safe to abandon: it may not hold anything that only an explicit stop would
// release.

// Iterator produces one pass over an iterable value.
type Iterator interface {
	// Next returns the next value and true, or false when the pass is over.
	// A non-nil error ends the pass and is raised at the consumer; Next is
	// not called again after it, nor after it reported false.
	Next() (Object, bool, error)
}

// Iterable is implemented by values that iterate lazily. Iterator starts a
// new pass; an error it returns is raised before the first element, as an
// error from Iter would be. A type implementing Iterable still implements
// Object.Iter, usually as Collect(self), for Go code that calls it directly.
type Iterable interface {
	Iterator() (Iterator, error)
}

// Iteration is the consumer's handle on one pass over a value, obtained from
// Iterate. It is a plain struct so iterating a List, or a value that only
// implements the eager Iter, allocates nothing beyond the elements: those
// passes walk a slice in place.
type Iteration struct {
	items []Object
	pos   int
	src   Iterator
}

// Iterate starts a pass over v: through Iterable when v implements it,
// otherwise over the elements v.Iter returns. A List is walked over the
// element slice it holds when the pass starts, as a Go range over it would:
// assigning an existing element shows up later in the pass, while push and
// pop do not change how many elements it visits.
func Iterate(v Object) (Iteration, error) {
	switch v := v.(type) {
	case *List:
		return Iteration{items: v.Elements}, nil
	case Iterable:
		src, err := v.Iterator()
		if err != nil {
			return Iteration{}, err
		}
		return Iteration{src: src}, nil
	}
	items, err := v.Iter()
	if err != nil {
		return Iteration{}, err
	}
	return Iteration{items: items}, nil
}

// Next returns the next value of the pass; see Iterator.Next.
func (it *Iteration) Next() (Object, bool, error) {
	if v, ok := it.Fast(); ok {
		return v, true, nil
	}
	return it.Pull()
}

// Fast returns the next element when the pass walks a slice and has one
// left; otherwise it returns false and the caller continues with Pull. The
// split keeps Fast small enough to inline, so a transpiled loop over a List
// costs an index and a bounds check per element.
func (it *Iteration) Fast() (Object, bool) {
	if it.pos < len(it.items) {
		v := it.items[it.pos]
		it.pos++
		return v, true
	}
	return nil, false
}

// Pull is Next after Fast reported false: the lazy source's next value, or
// the end of the pass.
func (it *Iteration) Pull() (Object, bool, error) {
	if it.src == nil {
		return nil, false, nil
	}
	return it.src.Next()
}

// IterationOf wraps a lazy iterator in an Iteration, for consumers that
// start a pass themselves rather than through Iterate.
func IterationOf(src Iterator) Iteration { return Iteration{src: src} }

// Collect runs a whole pass over v and returns its elements. For a List and
// for a value that only implements the eager Iter, the result is the slice
// Iter returns, which may be the value's own storage: callers must copy it
// before modifying it or keeping it.
func Collect(v Object) ([]Object, error) {
	it, err := Iterate(v)
	if err != nil {
		return nil, err
	}
	if it.src == nil {
		return it.items, nil
	}
	items := []Object{}
	for {
		item, ok, err := it.src.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return items, nil
		}
		items = append(items, item)
	}
}

// IteratorFunc adapts a Go function to Iterator.
type IteratorFunc func() (Object, bool, error)

func (f IteratorFunc) Next() (Object, bool, error) { return f() }
