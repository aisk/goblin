package object

import "strings"

// renderer formats the values nested inside a collection or a structural
// Show. It remembers the containers it is currently inside, so a value that
// contains itself renders as [...], {...} or Name(...) at the point of
// recursion instead of recursing until the Go stack overflows.
//
// A cycle that passes through a user-written show is not caught: that show
// starts a fresh rendering of its own.
type renderer struct {
	// strict selects ToString semantics: run Show and report its errors.
	// Otherwise rendering is infallible, as String must be.
	strict bool
	active []Object
}

// enter marks container as being rendered, reporting false when it already
// is, i.e. when rendering it again would recurse.
func (r *renderer) enter(container Object) bool {
	for _, a := range r.active {
		if a == container {
			return false
		}
	}
	r.active = append(r.active, container)
	return true
}

func (r *renderer) leave() { r.active = r.active[:len(r.active)-1] }

// element renders a value nested inside another: strings are quoted, and
// containers recurse through the same renderer.
func (r *renderer) element(obj Object) (string, error) {
	switch v := obj.(type) {
	case String:
		return v.Literal(), nil
	case *List:
		return r.list(v)
	case *Dict:
		return r.dict(v)
	case UserValue:
		if hasStructuralShow(v) {
			return r.user(v)
		}
	}
	if r.strict {
		return obj.ToString()
	}
	return inspect(obj), nil
}

func (r *renderer) list(l *List) (string, error) {
	if !r.enter(l) {
		return "[...]", nil
	}
	defer r.leave()
	var b strings.Builder
	b.WriteByte('[')
	for i, elem := range l.Elements {
		if i > 0 {
			b.WriteString(", ")
		}
		s, err := r.element(elem)
		if err != nil {
			return "", err
		}
		b.WriteString(s)
	}
	b.WriteByte(']')
	return b.String(), nil
}

func (r *renderer) dict(d *Dict) (string, error) {
	if !r.enter(d) {
		return "{...}", nil
	}
	defer r.leave()
	var b strings.Builder
	b.WriteByte('{')
	for i, entry := range d.Entries() {
		if i > 0 {
			b.WriteString(", ")
		}
		key, err := r.element(entry.Key)
		if err != nil {
			return "", err
		}
		value, err := r.element(entry.Value)
		if err != nil {
			return "", err
		}
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(value)
	}
	b.WriteByte('}')
	return b.String(), nil
}

// user renders `Name(field=value, ...)`, the structural Show.
func (r *renderer) user(v UserValue) (string, error) {
	if !r.enter(v) {
		return v.TypeName() + "(...)", nil
	}
	defer r.leave()
	var b strings.Builder
	b.WriteString(v.TypeName())
	b.WriteByte('(')
	names := v.UserType().Fields
	for i, value := range v.FieldValues() {
		if i > 0 {
			b.WriteString(", ")
		}
		s, err := r.element(value)
		if err != nil {
			return "", err
		}
		b.WriteString(names[i])
		b.WriteByte('=')
		b.WriteString(s)
	}
	b.WriteByte(')')
	return b.String(), nil
}

// hasStructuralShow reports whether v's type got its Show from an empty
// `impl Show {}`, whose rendering the renderer can do itself.
func hasStructuralShow(v UserValue) bool {
	impl := v.UserType().show
	return impl != nil && impl.methods[ShowShow] == ShowTrait.structural[ShowShow]
}
