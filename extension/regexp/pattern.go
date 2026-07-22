package regexp

import (
	"fmt"
	stdregexp "regexp"

	"github.com/aisk/goblin/object"
)

// Pattern is an immutable, concurrency-safe compiled regular expression.
type Pattern struct {
	objectBase
	source string
	re     *stdregexp.Regexp
	full   *stdregexp.Regexp
}

func (p *Pattern) String() string              { return fmt.Sprintf("<regexp.Pattern %q>", p.source) }
func (p *Pattern) ToString() (string, error)   { return p.String(), nil }
func (p *Pattern) Bool() bool                  { return true }
func (p *Pattern) ToBool() (bool, error)       { return true, nil }
func (p *Pattern) Not() (object.Object, error) { return object.False, nil }
func (p *Pattern) Equals(other object.Object) bool {
	v, ok := other.(*Pattern)
	return ok && p.source == v.source
}

func (p *Pattern) matcher(full object.Bool) *stdregexp.Regexp {
	if full {
		return p.full
	}
	return p.re
}

func parseTextFull(name string, args object.CallArgs) (string, object.Bool, error) {
	ap := object.NewArgParser(name, args)
	text := ap.Str("text")
	full := ap.BoolOr("full", object.False)
	if err := ap.Finish(); err != nil {
		return "", false, err
	}
	return string(text), full, nil
}

func (p *Pattern) test(args object.CallArgs) (object.Object, error) {
	text, full, err := parseTextFull("test", args)
	if err != nil {
		return nil, err
	}
	return object.Bool(p.matcher(full).MatchString(text)), nil
}

func (p *Pattern) find(args object.CallArgs) (object.Object, error) {
	text, full, err := parseTextFull("find", args)
	if err != nil {
		return nil, err
	}
	indices := p.matcher(full).FindStringSubmatchIndex(text)
	if indices == nil {
		return object.Nil, nil
	}
	return newMatch(text, indices, p.re.SubexpNames()), nil
}

func parseTextLimit(name string, args object.CallArgs) (string, int, error) {
	ap := object.NewArgParser(name, args)
	text := ap.Str("text")
	limit := ap.IntOr("limit", -1)
	if err := ap.Finish(); err != nil {
		return "", 0, err
	}
	if limit < -1 {
		return "", 0, object.NewValueError("%s() limit must be -1 or non-negative", name)
	}
	return string(text), int(limit), nil
}

func (p *Pattern) findAll(args object.CallArgs) (object.Object, error) {
	text, limit, err := parseTextLimit("find_all", args)
	if err != nil {
		return nil, err
	}
	if limit == 0 {
		return &object.List{Elements: []object.Object{}}, nil
	}
	indices := p.re.FindAllStringSubmatchIndex(text, limit)
	items := make([]object.Object, len(indices))
	for i, index := range indices {
		items[i] = newMatch(text, index, p.re.SubexpNames())
	}
	return &object.List{Elements: items}, nil
}

func (p *Pattern) replace(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("replace", args)
	text := ap.Str("text")
	replacement := ap.Str("replacement")
	limit := ap.IntOr("limit", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if limit < -1 {
		return nil, object.NewValueError("replace() limit must be -1 or non-negative")
	}
	if limit == 0 {
		return text, nil
	}
	s := string(text)
	matches := p.re.FindAllStringSubmatchIndex(s, int(limit))
	result := make([]byte, 0, len(s))
	last := 0
	for _, match := range matches {
		result = append(result, s[last:match[0]]...)
		result = p.re.ExpandString(result, string(replacement), s, match)
		last = match[1]
	}
	result = append(result, s[last:]...)
	return object.String(result), nil
}

func (p *Pattern) split(args object.CallArgs) (object.Object, error) {
	text, limit, err := parseTextLimit("split", args)
	if err != nil {
		return nil, err
	}
	if limit == 0 {
		return &object.List{Elements: []object.Object{object.String(text)}}, nil
	}
	splits := p.re.Split(text, limit+1)
	if limit < 0 {
		splits = p.re.Split(text, -1)
	}
	items := make([]object.Object, len(splits))
	for i, value := range splits {
		items[i] = object.String(value)
	}
	return &object.List{Elements: items}, nil
}

func (p *Pattern) GetAttr(name string) (object.Object, error) {
	methods := map[string]func(object.CallArgs) (object.Object, error){
		"test": p.test, "find": p.find, "find_all": p.findAll,
		"replace": p.replace, "split": p.split,
	}
	if name == "attributes" {
		return object.AttributesFunction(p), nil
	}
	if fn, ok := methods[name]; ok {
		return &object.Function{Name: name, Fn: fn}, nil
	}
	return nil, object.NewAttributeError("Pattern has no attribute '%s'", name)
}

func (p *Pattern) Attributes() []string {
	return []string{"attributes", "test", "find", "find_all", "replace", "split"}
}

var _ object.Object = (*Pattern)(nil)
