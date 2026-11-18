package object

import (
	"errors"
	"fmt"
	"strings"
)

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
// A consumer may stop pulling at any point (break, return, an error in the
// body), so it tells the iterator through Iteration.Close: whoever starts a
// pass with Iterate calls Close exactly once however the pass ends, and never
// calls Next after it. An iterator holding something for the duration of one
// pass implements Closer and releases it there, and only there.
//
// Ownership: one pass owns what it acquired when it started; what the
// iterable itself owns is not the pass's to release. Iterating a file leaves
// the file open, while an iterable that opens a file in Iterator closes it in
// Close, so iterating it again opens it again.

// Iterator produces one pass over an iterable value.
type Iterator interface {
	// Next returns the next value and true, or false when the pass is over.
	// A non-nil error ends the pass and is raised at the consumer; Next is
	// not called again after it, nor after it reported false.
	Next() (Object, bool, error)
}

// Closer is optionally implemented by an Iterator that holds something for
// the duration of one pass. Close cannot fail: a source whose release can
// fail reports that from its last Next.
type Closer interface {
	Close()
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
	list  *List
	n     int
	pos   int
	src   Iterator
}

// Iterate starts a pass over v: through Iterable when v implements it,
// otherwise over the elements v.Iter returns. A List is walked in place, by
// position, over at most the number of elements it had when the pass
// started: assigning an existing element shows up later in the pass,
// elements pushed during the pass are not visited, and the pass ends early
// when the list has become shorter than the position it reached. It never
// copies the list and never hands out a slot that a pop or remove vacated.
func Iterate(v Object) (Iteration, error) {
	switch v := v.(type) {
	case *List:
		return Iteration{list: v, n: len(v.Elements)}, nil
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
// costs an index and two bounds checks per element.
func (it *Iteration) Fast() (Object, bool) {
	if l := it.list; l != nil {
		if it.pos < it.n && it.pos < len(l.Elements) {
			v := l.Elements[it.pos]
			it.pos++
			return v, true
		}
		return nil, false
	}
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

// Close ends the pass, closing its lazy source when that implements Closer.
// It is idempotent, and a no-op for a pass over a slice; the split keeps it
// small enough to inline into every for loop.
func (it *Iteration) Close() {
	if it.src != nil {
		it.closeSrc()
	}
}

func (it *Iteration) closeSrc() {
	src := it.src
	it.src = nil
	if c, ok := src.(Closer); ok {
		c.Close()
	}
}

// IterationOf wraps a lazy iterator in an Iteration, for consumers that
// start a pass themselves rather than through Iterate.
func IterationOf(src Iterator) Iteration { return Iteration{src: src} }

// Collect runs a whole pass over v and returns its elements. For a List and
// for a value that only implements the eager Iter, the result is the slice
// Iter returns, which may be the value's own storage: callers must copy it
// before modifying it or keeping it, and a caller that runs Goblin code while
// it reads the slice (which may pop from the list) uses CollectCopy instead.
func Collect(v Object) ([]Object, error) {
	it, err := Iterate(v)
	if err != nil {
		return nil, err
	}
	if it.list != nil {
		return it.list.Elements, nil
	}
	if it.src == nil {
		return it.items, nil
	}
	defer it.Close()
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

// IteratorWithClose adapts a pair of Go functions to an Iterator that is
// also a Closer: next produces the values, close releases what the pass
// holds.
func IteratorWithClose(next func() (Object, bool, error), close func()) Iterator {
	return &closingIterator{next: next, close: close}
}

type closingIterator struct {
	next  func() (Object, bool, error)
	close func()
}

func (it *closingIterator) Next() (Object, bool, error) { return it.next() }
func (it *closingIterator) Close()                      { it.close() }

// CollectArg is Collect for the argument param of the builtin fn: a value
// that is not iterable at all is reported as "fn() argument 'param' must be
// iterable, got Int", while an error raised by the iteration itself passes
// through unchanged.
func CollectArg(fn, param string, v Object) ([]Object, error) {
	items, err := Collect(v)
	if err != nil {
		return nil, iterableArgError(fn, param, v, err)
	}
	return items, nil
}

// IterateArg is Iterate for the argument param of the builtin fn, reporting
// a value that is not iterable as CollectArg does. A builtin that runs Goblin
// code for each element (a key, a user Add) walks with it rather than with
// CollectArg, so a List changed by that code follows the rule of Iterate.
// An empty param leaves the parameter unnamed: "fn() argument must be
// iterable, got Int", for a builtin that takes the value only by position.
func IterateArg(fn, param string, v Object) (Iteration, error) {
	it, err := Iterate(v)
	if err != nil {
		return it, iterableArgError(fn, param, v, err)
	}
	return it, nil
}

func iterableArgError(fn, param string, v Object, err error) error {
	var e *Error
	// A user type's message carries a hint after the shared text.
	if errors.As(err, &e) && errors.Is(err, TypeError) && strings.HasPrefix(e.Value, fmt.Sprintf(ErrFmtNotIterable, v.TypeName())) {
		if param == "" {
			return NewTypeError("%s() argument must be iterable, got %s", fn, v.TypeName())
		}
		return NewTypeError("%s() argument '%s' must be iterable, got %s", fn, param, v.TypeName())
	}
	return err
}
