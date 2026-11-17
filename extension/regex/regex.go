// Package regex is the regex module: search and rewrite text by pattern. A
// pattern is a String in RE2 syntax (Go's regexp); there is no compiled
// pattern object, because patterns are compiled on first use and kept in a
// small cache.
package regex

import (
	"container/list"
	stdregexp "regexp"
	"strings"
	"sync"

	"github.com/aisk/goblin/object"
)

// Execute builds the regex standard-library module.
func Execute() (object.Object, error) {
	return object.NewModule("regex", map[string]object.Object{
		"Match": MatchType.Function,
	}, object.Funcs{
		"find":     find,
		"find_all": findAll,
		"replace":  replace,
		"split":    split,
		"escape":   escape,
	}), nil
}

// MatchType is the type name regex exports for Match; matches come from
// the module's functions.
var MatchType = object.NewOpaqueConstructor("Match", "regex.find(pattern, text)")

// cacheSize is how many compiled patterns are remembered, most recent first.
const cacheSize = 256

type cacheKey struct {
	pattern string
	full    bool
}

type cacheEntry struct {
	key cacheKey
	re  *stdregexp.Regexp
}

// cache is an LRU of compiled patterns. The mutex is held only for the
// lookup and the insert; compiling happens outside it, and a compiled
// *regexp.Regexp is safe for concurrent use.
var cache = struct {
	sync.Mutex
	order *list.List // of *cacheEntry, most recent at the front
	index map[cacheKey]*list.Element
}{order: list.New(), index: map[cacheKey]*list.Element{}}

func cached(key cacheKey) (*stdregexp.Regexp, bool) {
	cache.Lock()
	defer cache.Unlock()
	if e, ok := cache.index[key]; ok {
		cache.order.MoveToFront(e)
		return e.Value.(*cacheEntry).re, true
	}
	return nil, false
}

func remember(key cacheKey, re *stdregexp.Regexp) {
	cache.Lock()
	defer cache.Unlock()
	if e, ok := cache.index[key]; ok {
		cache.order.MoveToFront(e)
		return
	}
	cache.index[key] = cache.order.PushFront(&cacheEntry{key: key, re: re})
	if cache.order.Len() > cacheSize {
		oldest := cache.order.Back()
		cache.order.Remove(oldest)
		delete(cache.index, oldest.Value.(*cacheEntry).key)
	}
}

// compile returns the compiled pattern, anchored at both ends for full. A
// pattern that fails to compile is a ParseError carrying the pattern as
// given, and is not remembered.
func compile(fn, pattern string, full bool) (*stdregexp.Regexp, error) {
	key := cacheKey{pattern, full}
	if re, ok := cached(key); ok {
		return re, nil
	}
	source := pattern
	if full {
		// Compile the plain pattern first, so an error names what the user
		// wrote rather than the anchored form.
		if _, err := compile(fn, pattern, false); err != nil {
			return nil, err
		}
		source = `\A(?:` + pattern + `)\z`
	}
	re, err := stdregexp.Compile(source)
	if err != nil {
		return nil, object.NewParseError("%s() invalid pattern: %s", fn, strings.TrimPrefix(err.Error(), "error parsing regexp: ")).
			With("pattern", object.String(pattern))
	}
	remember(key, re)
	return re, nil
}

// countArg reads the optional count argument as a limit for the Go engine,
// where -1 means none, finishing the parse.
func countArg(fn string, ap *object.ArgParser) (int, error) {
	count, limited := ap.NullableInt("count")
	if err := ap.Finish(); err != nil {
		return 0, err
	}
	if !limited {
		return -1, nil
	}
	if count < 0 {
		return 0, object.NewValueError("%s() count must not be negative", fn)
	}
	return int(count), nil
}

func find(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("find", args)
	pattern := ap.Str("pattern")
	text := ap.Str("text")
	full := ap.BoolOr("full", object.False)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	re, err := compile("find", string(pattern), bool(full))
	if err != nil {
		return nil, err
	}
	indices := re.FindStringSubmatchIndex(string(text))
	if indices == nil {
		return object.Nil, nil
	}
	return newMatch(string(text), indices, re.SubexpNames()), nil
}

func findAll(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("find_all", args)
	pattern := ap.Str("pattern")
	text := ap.Str("text")
	count, err := countArg("find_all", ap)
	if err != nil {
		return nil, err
	}
	re, err := compile("find_all", string(pattern), false)
	if err != nil {
		return nil, err
	}
	names := re.SubexpNames()
	all := re.FindAllStringSubmatchIndex(string(text), count)
	items := make([]object.Object, len(all))
	for i, indices := range all {
		items[i] = newMatch(string(text), indices, names)
	}
	return &object.List{Elements: items}, nil
}

func replace(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("replace", args)
	pattern := ap.Str("pattern")
	text := ap.Str("text")
	replacement := ap.Any("replacement")
	count, err := countArg("replace", ap)
	if err != nil {
		return nil, err
	}
	template, isTemplate := replacement.(object.String)
	fn, isFunc := replacement.(*object.Function)
	if !isTemplate && !isFunc {
		return nil, object.NewTypeError("replace() argument 'replacement' must be String or Function, got %s", replacement.TypeName())
	}
	re, err := compile("replace", string(pattern), false)
	if err != nil {
		return nil, err
	}
	names := re.SubexpNames()
	if isTemplate {
		if err := checkTemplate(names, string(template)); err != nil {
			return nil, err
		}
	}
	s := string(text)
	matches := re.FindAllStringSubmatchIndex(s, count)
	result := make([]byte, 0, len(s))
	last := 0
	for _, match := range matches {
		result = append(result, s[last:match[0]]...)
		if isTemplate {
			result = re.ExpandString(result, string(template), s, match)
		} else {
			// Called on the calling goblin; an error it raises ends replace.
			got, err := fn.Call(object.CallArgs{Positional: object.Args{newMatch(s, match, names)}})
			if err != nil {
				return nil, err
			}
			piece, ok := got.(object.String)
			if !ok {
				return nil, object.NewTypeError("replace() replacement function must return String, got %s", got.TypeName())
			}
			result = append(result, piece...)
		}
		last = match[1]
	}
	result = append(result, s[last:]...)
	return object.String(result), nil
}

func split(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("split", args)
	pattern := ap.Str("pattern")
	text := ap.Str("text")
	count, err := countArg("split", ap)
	if err != nil {
		return nil, err
	}
	re, err := compile("split", string(pattern), false)
	if err != nil {
		return nil, err
	}
	// count limits the splits, so the pieces number at most one more.
	if count >= 0 {
		count++
	}
	pieces := re.Split(string(text), count)
	items := make([]object.Object, len(pieces))
	for i, value := range pieces {
		items[i] = object.String(value)
	}
	return &object.List{Elements: items}, nil
}

// escape quotes every metacharacter in text, so that the result matches text
// literally. It is the safe way to build a pattern around input that is not
// itself a pattern.
func escape(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("escape", args)
	text := p.Str("text")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.String(stdregexp.QuoteMeta(string(text))), nil
}
