package regexp

import (
	"fmt"
	stdregexp "regexp"
	"sync"

	"github.com/aisk/goblin/object"
)

// Pattern is an immutable, concurrency-safe compiled regular expression.
type Pattern struct {
	objectBase
	source string
	re     *stdregexp.Regexp
	// names is re.SubexpNames(): entry i holds the name of group i, or "" when
	// that group is unnamed. It is never mutated, so Match values share it.
	names []string
	// full anchors the source at both ends for match(full=true). Most patterns
	// never need it, so it is compiled on first use.
	fullOnce sync.Once
	full     *stdregexp.Regexp
	fullErr  error
}

func (p *Pattern) String() string            { return fmt.Sprintf("<regexp.Pattern %q>", p.source) }
func (p *Pattern) ToString() (string, error) { return p.String(), nil }

// Equals compares by source. Two patterns compiled from the same text behave
// identically, so they are interchangeable values rather than handles.
func (p *Pattern) Equals(other object.Object) (bool, error) {
	v, ok := other.(*Pattern)
	return ok && p.source == v.source, nil
}

// matcher returns the engine for a match request. The full=true engine wraps
// the source in a non-capturing group, so group numbers and names are the same
// in both engines and Match can always be built from p.names.
func (p *Pattern) matcher(full object.Bool) (*stdregexp.Regexp, error) {
	if !full {
		return p.re, nil
	}
	p.fullOnce.Do(func() {
		p.full, p.fullErr = stdregexp.Compile(`\A(?:` + p.source + `)\z`)
	})
	if p.fullErr != nil {
		return nil, object.WrapError(object.ParseError, "anchoring the pattern for full=true failed", p.fullErr)
	}
	return p.full, nil
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

func (p *Pattern) match(args object.CallArgs) (object.Object, error) {
	text, full, err := parseTextFull("match", args)
	if err != nil {
		return nil, err
	}
	re, err := p.matcher(full)
	if err != nil {
		return nil, err
	}
	return object.Bool(re.MatchString(text)), nil
}

func (p *Pattern) find(args object.CallArgs) (object.Object, error) {
	text, full, err := parseTextFull("find", args)
	if err != nil {
		return nil, err
	}
	re, err := p.matcher(full)
	if err != nil {
		return nil, err
	}
	indices := re.FindStringSubmatchIndex(text)
	if indices == nil {
		return object.Nil, nil
	}
	return newMatch(text, indices, p.names), nil
}

// parseTextCount reads the (text, count) argument pair shared by find_all and
// split. count follows the built-in str methods: any negative value means "no
// limit", and it is passed straight through to the Go engine.
func parseTextCount(name string, args object.CallArgs) (string, int, error) {
	ap := object.NewArgParser(name, args)
	text := ap.Str("text")
	count := ap.IntOr("count", -1)
	if err := ap.Finish(); err != nil {
		return "", 0, err
	}
	return string(text), int(count), nil
}

func (p *Pattern) findAll(args object.CallArgs) (object.Object, error) {
	text, count, err := parseTextCount("find_all", args)
	if err != nil {
		return nil, err
	}
	indices := p.re.FindAllStringSubmatchIndex(text, count)
	items := make([]object.Object, len(indices))
	for i, index := range indices {
		items[i] = newMatch(text, index, p.names)
	}
	return &object.List{Elements: items}, nil
}

func (p *Pattern) replace(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("replace", args)
	text := ap.Str("text")
	replacement := ap.Str("replacement")
	count := ap.IntOr("count", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if err := p.checkTemplate(string(replacement)); err != nil {
		return nil, err
	}
	s := string(text)
	matches := p.re.FindAllStringSubmatchIndex(s, int(count))
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
	text, count, err := parseTextCount("split", args)
	if err != nil {
		return nil, err
	}
	pieces := p.re.Split(text, count)
	items := make([]object.Object, len(pieces))
	for i, value := range pieces {
		items[i] = object.String(value)
	}
	return &object.List{Elements: items}, nil
}

// groupNames lists capture-group names by number, excluding group 0. Entry i
// holds the name of group i+1, or nil when that group is unnamed, so it lines
// up element for element with Match.groups.
func (p *Pattern) groupNames() *object.List {
	items := make([]object.Object, len(p.names)-1)
	for i, name := range p.names[1:] {
		if name == "" {
			items[i] = object.Nil
			continue
		}
		items[i] = object.String(name)
	}
	return &object.List{Elements: items}
}

// hasGroupName reports whether the pattern declares a group called name.
func (p *Pattern) hasGroupName(name string) bool {
	for _, candidate := range p.names[1:] {
		if candidate != "" && candidate == name {
			return true
		}
	}
	return false
}

func (p *Pattern) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(p), nil
	case "pattern":
		return object.String(p.source), nil
	case "group_names":
		return p.groupNames(), nil
	}
	methods := map[string]func(object.CallArgs) (object.Object, error){
		"match": p.match, "find": p.find, "find_all": p.findAll,
		"replace": p.replace, "split": p.split,
	}
	if fn, ok := methods[name]; ok {
		return &object.Function{Name: name, Fn: fn}, nil
	}
	return nil, object.NewAttributeError("Pattern has no attribute '%s'", name)
}

func (p *Pattern) Attributes() []string {
	return []string{
		"attributes", "pattern", "group_names",
		"match", "find", "find_all", "replace", "split",
	}
}

var _ object.Object = (*Pattern)(nil)
