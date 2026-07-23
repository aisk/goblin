package regexp

import (
	"fmt"
	stdregexp "regexp"

	"github.com/aisk/goblin/object"
)

// Regexp wraps Go's immutable, concurrency-safe regexp.Regexp.
type Regexp struct {
	objectBase
	source string
	re     *stdregexp.Regexp
}

func (r *Regexp) String() string              { return fmt.Sprintf("<regexp.Regexp %q>", r.source) }
func (r *Regexp) ToString() (string, error)   { return r.source, nil }
func (r *Regexp) Bool() bool                  { return true }
func (r *Regexp) ToBool() (bool, error)       { return true, nil }
func (r *Regexp) Not() (object.Object, error) { return object.False, nil }
func (r *Regexp) Equals(other object.Object) bool {
	value, ok := other.(*Regexp)
	return ok && r.source == value.source
}

func (r *Regexp) matchString(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("match_string", args)
	text := p.Str("text")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Bool(r.re.MatchString(string(text))), nil
}

// findString combines Go's Find(All)?String(Submatch)?(Index)? family through
// Goblin keyword arguments. n follows Go's FindAll convention and is used only
// when all=true.
func (r *Regexp) findString(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("find_string", args)
	text := p.Str("text")
	all := p.BoolOr("all", object.False)
	submatch := p.BoolOr("submatch", object.False)
	index := p.BoolOr("index", object.False)
	n := p.IntOr("n", -1)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if !all && n != -1 {
		return nil, object.NewTypeError("find_string() argument 'n' requires all=true")
	}
	s := string(text)
	if all {
		switch {
		case bool(submatch) && bool(index):
			return intMatrix(r.re.FindAllStringSubmatchIndex(s, int(n))), nil
		case bool(submatch):
			return stringMatrix(r.re.FindAllStringSubmatch(s, int(n))), nil
		case bool(index):
			return intMatrix(r.re.FindAllStringIndex(s, int(n))), nil
		default:
			return stringList(r.re.FindAllString(s, int(n))), nil
		}
	}
	switch {
	case bool(submatch) && bool(index):
		return optionalIntList(r.re.FindStringSubmatchIndex(s)), nil
	case bool(submatch):
		return optionalStringList(r.re.FindStringSubmatch(s)), nil
	case bool(index):
		return optionalIntList(r.re.FindStringIndex(s)), nil
	default:
		return object.String(r.re.FindString(s)), nil
	}
}

func (r *Regexp) replaceAllString(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("replace_all_string", args)
	text := p.Str("text")
	replacement := p.Str("replacement")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.String(r.re.ReplaceAllString(string(text), string(replacement))), nil
}

func (r *Regexp) split(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("split", args)
	text := p.Str("text")
	n := p.IntOr("n", -1)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return stringList(r.re.Split(string(text), int(n))), nil
}

func (r *Regexp) subexpNames(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("subexp_names", args)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return stringList(r.re.SubexpNames()), nil
}

func (r *Regexp) GetAttr(name string) (object.Object, error) {
	methods := map[string]func(object.CallArgs) (object.Object, error){
		"match_string":       r.matchString,
		"find_string":        r.findString,
		"replace_all_string": r.replaceAllString,
		"split":              r.split,
		"subexp_names":       r.subexpNames,
	}
	if name == "attributes" {
		return object.AttributesFunction(r), nil
	}
	if fn, ok := methods[name]; ok {
		return &object.Function{Name: name, Fn: fn}, nil
	}
	return nil, object.NewAttributeError("Regexp has no attribute '%s'", name)
}

func (r *Regexp) Attributes() []string {
	return []string{"attributes", "match_string", "find_string", "replace_all_string", "split", "subexp_names"}
}

func stringList(values []string) *object.List {
	items := make([]object.Object, len(values))
	for i, value := range values {
		items[i] = object.String(value)
	}
	return &object.List{Elements: items}
}

func intList(values []int) *object.List {
	items := make([]object.Object, len(values))
	for i, value := range values {
		items[i] = object.Integer(value)
	}
	return &object.List{Elements: items}
}

func optionalStringList(values []string) object.Object {
	if values == nil {
		return object.Nil
	}
	return stringList(values)
}

func optionalIntList(values []int) object.Object {
	if values == nil {
		return object.Nil
	}
	return intList(values)
}

func stringMatrix(values [][]string) *object.List {
	items := make([]object.Object, len(values))
	for i, value := range values {
		items[i] = stringList(value)
	}
	return &object.List{Elements: items}
}

func intMatrix(values [][]int) *object.List {
	items := make([]object.Object, len(values))
	for i, value := range values {
		items[i] = intList(value)
	}
	return &object.List{Elements: items}
}

var _ object.Object = (*Regexp)(nil)
