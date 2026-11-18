package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/aisk/goblin/object"
)

// scanner reads a command line against a declaration, left to right.
type scanner struct {
	program string
	root    *command
	cmd     *command // the subcommand named on the line, once it is
	argPos  int      // the next Arg of the current target to fill
	given   map[*param]object.Object
	lists   map[*param][]object.Object
	counts  map[*param]int
}

// looksLikeOption reports whether a token is an option: it starts with a
// dash, is not "-" alone, and is not a number such as -5, -1.5 or -.5.
func looksLikeOption(t string) bool {
	if len(t) < 2 || t[0] != '-' {
		return false
	}
	if isDigit(t[1]) || (t[1] == '.' && len(t) > 2 && isDigit(t[2])) {
		return false
	}
	return true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// active is the options accepted at this point: the global ones, and the
// subcommand's after it has been named.
func (s *scanner) active() []*param {
	if s.cmd == nil {
		return s.root.options
	}
	return append(append([]*param(nil), s.root.options...), s.cmd.options...)
}

func (s *scanner) target() *command {
	if s.cmd != nil {
		return s.cmd
	}
	return s.root
}

func (s *scanner) usageError(format string, a ...any) error {
	helpCommand := s.program + " --help"
	if s.cmd != nil {
		helpCommand = s.program + " " + s.cmd.long() + " --help"
	}
	return &stopError{text: fmtMessage(format, a...), usage: usageLine(s.program, s.root, s.cmd), helpCommand: helpCommand}
}

func fmtMessage(format string, a ...any) string {
	return fmt.Sprintf(format, a...)
}

func (s *scanner) help() error {
	return &stopError{help: true, text: helpText(s.program, s.root, s.cmd)}
}

// scan parses argv against root and returns the Args, or a stopError for a
// usage error or a request for help; any other error is the program's own
// (a convert function failing in an unexpected way).
func scan(program string, root *command, argv []string) (object.Object, error) {
	s := &scanner{program: program, root: root,
		given: map[*param]object.Object{}, lists: map[*param][]object.Object{}, counts: map[*param]int{}}
	positionalOnly := false
	for i := 0; i < len(argv); {
		token := argv[i]
		i++
		switch {
		case positionalOnly || !looksLikeOption(token):
			if err := s.positional(token); err != nil {
				return nil, err
			}
		case token == "--":
			positionalOnly = true
		case strings.HasPrefix(token, "--"):
			name, value, hasValue := strings.Cut(token[2:], "=")
			p := s.long(name)
			if p == nil {
				if name == "help" && !hasValue {
					return nil, s.help()
				}
				if name == "help" {
					return nil, s.usageError("option --help takes no value")
				}
				return nil, s.unknownLong(name)
			}
			if p.kind == kindFlag {
				if hasValue {
					return nil, s.usageError("option %s takes no value", p.label())
				}
				s.counts[p]++
				continue
			}
			if !hasValue {
				if i >= len(argv) || looksLikeOption(argv[i]) {
					return nil, s.usageError("option %s needs a value", p.label())
				}
				value = argv[i]
				i++
			}
			if err := s.assign(p, value); err != nil {
				return nil, err
			}
		default:
			letters := token[1:]
			for j := 0; j < len(letters); j++ {
				c := letters[j]
				p := s.short(c)
				if p == nil {
					if c == 'h' {
						return nil, s.help()
					}
					return nil, s.usageError("unknown option -%s", string(c))
				}
				if p.kind == kindFlag {
					s.counts[p]++
					continue
				}
				var value string
				if j+1 < len(letters) {
					value = strings.TrimPrefix(letters[j+1:], "=")
				} else {
					if i >= len(argv) || looksLikeOption(argv[i]) {
						return nil, s.usageError("option %s needs a value", p.label())
					}
					value = argv[i]
					i++
				}
				if err := s.assign(p, value); err != nil {
					return nil, err
				}
				break
			}
		}
	}
	return s.finish()
}

func (s *scanner) long(name string) *param {
	for _, p := range s.active() {
		if p.long() == name {
			return p
		}
	}
	return nil
}

// short finds the option a letter names; 'h' is help unless claimed.
func (s *scanner) short(c byte) *param {
	for _, p := range s.active() {
		if p.short == c {
			return p
		}
	}
	return nil
}

func (s *scanner) unknownLong(name string) error {
	var names []string
	for _, p := range s.active() {
		names = append(names, p.long())
	}
	if guess, ok := nearest(name, names); ok {
		return s.usageError("unknown option --%s (did you mean --%s?)", name, guess)
	}
	return s.usageError("unknown option --%s", name)
}

// nearest is the candidate within edit distance 2 of word, the earliest
// on a tie.
func nearest(word string, candidates []string) (string, bool) {
	best, bestDistance := "", 3
	for _, c := range candidates {
		if d := editDistance(word, c); d < bestDistance {
			best, bestDistance = c, d
		}
	}
	return best, bestDistance <= 2
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

// positional takes a token that is not an option: the command name while
// one is expected, else the next Arg.
func (s *scanner) positional(token string) error {
	if len(s.root.commands) > 0 && s.cmd == nil {
		var names []string
		for _, c := range s.root.commands {
			if c.long() == token {
				s.cmd = c
				return nil
			}
			names = append(names, c.long())
		}
		if guess, ok := nearest(token, names); ok {
			return s.usageError("unknown command %s (did you mean %s?)", strconv.Quote(token), guess)
		}
		return s.usageError("unknown command %s", strconv.Quote(token))
	}
	args := s.target().args
	if s.argPos >= len(args) {
		return s.usageError("unexpected argument %s", strconv.Quote(token))
	}
	p := args[s.argPos]
	if !p.many {
		s.argPos++
	}
	return s.assign(p, token)
}

// assign converts one value of p, checks its choices and records it.
func (s *scanner) assign(p *param, text string) error {
	v, err := s.convert(p, text)
	if err != nil {
		return err
	}
	if len(p.choices) > 0 {
		found := false
		for _, c := range p.choices {
			if eq, err := object.Equals(v, c); err != nil {
				return err
			} else if eq {
				found = true
				break
			}
		}
		if !found {
			names := make([]string, len(p.choices))
			for i, c := range p.choices {
				names[i] = plain(c)
			}
			return s.usageError("invalid value %s for %s: expected one of %s", strconv.Quote(text), p.label(), strings.Join(names, ", "))
		}
	}
	if p.many {
		s.lists[p] = append(s.lists[p], v)
	} else {
		s.given[p] = v // the last one wins
	}
	return nil
}

// convert turns the text of a value into what p holds: its convert
// function's result, else an Int or Float when its default is one, else
// the text.
func (s *scanner) convert(p *param, text string) (object.Object, error) {
	conv := p.convert
	if conv == nil {
		switch p.def.(type) {
		case object.Integer:
			conv = object.IntConstructorFn
		case object.Float:
			conv = object.FloatConstructorFn
		default:
			return object.String(text), nil
		}
	}
	v, err := conv.Call(object.CallArgs{Positional: object.Args{object.String(text)}})
	if err == nil {
		return v, nil
	}
	// A number too large for an Int is an OverflowError from Int, but to
	// the user it is a value that is not a valid integer like any other.
	overflow := conv == object.IntConstructorFn && errors.Is(err, object.OverflowError)
	if !errors.Is(err, object.ValueError) && !overflow {
		return nil, err
	}
	switch conv {
	case object.IntConstructorFn:
		return nil, s.usageError("invalid value %s for %s: expected an integer", strconv.Quote(text), p.label())
	case object.FloatConstructorFn:
		return nil, s.usageError("invalid value %s for %s: expected a number", strconv.Quote(text), p.label())
	}
	return nil, s.usageError("invalid value %s for %s: %s", strconv.Quote(text), p.label(), err.Error())
}

// finish checks what is missing and builds the Args.
func (s *scanner) finish() (object.Object, error) {
	if len(s.root.commands) > 0 && s.cmd == nil {
		names := make([]string, len(s.root.commands))
		for i, c := range s.root.commands {
			names[i] = c.long()
		}
		return nil, s.usageError("missing command (one of %s)", strings.Join(names, ", "))
	}
	var missing []string
	for _, p := range s.target().args {
		if p.hasDefault {
			continue
		}
		if _, given := s.given[p]; p.many && len(s.lists[p]) == 0 || !p.many && !given {
			missing = append(missing, p.label())
		}
	}
	if len(missing) == 1 {
		return nil, s.usageError("missing argument %s", missing[0])
	}
	if len(missing) > 1 {
		return nil, s.usageError("missing arguments %s", strings.Join(missing, " "))
	}
	for _, p := range s.active() {
		if _, given := s.given[p]; p.required && !given && len(s.lists[p]) == 0 {
			return nil, s.usageError("missing required option %s", p.label())
		}
	}
	result := &Args{OpaqueBase: object.MakeOpaqueBase("Args"), values: map[string]object.Object{}}
	if s.cmd != nil {
		result.set("command", object.String(s.cmd.long()))
		for _, p := range s.root.options {
			result.set(p.name, s.value(p))
		}
		for _, p := range s.cmd.args {
			result.set(p.name, s.value(p))
		}
		for _, p := range s.cmd.options {
			result.set(p.name, s.value(p))
		}
		return result, nil
	}
	for _, p := range s.root.args {
		result.set(p.name, s.value(p))
	}
	for _, p := range s.root.options {
		result.set(p.name, s.value(p))
	}
	return result, nil
}

// value is what p holds after the scan.
func (s *scanner) value(p *param) object.Object {
	switch {
	case p.kind == kindFlag && p.many:
		return object.Integer(s.counts[p])
	case p.kind == kindFlag:
		return object.Bool(s.counts[p] > 0)
	case p.many:
		if values, ok := s.lists[p]; ok {
			return &object.List{Elements: values}
		}
		if p.hasDefault {
			// A new List on every parse, so changing it changes nothing else.
			return &object.List{Elements: append([]object.Object{}, p.def.(*object.List).Elements...)}
		}
		return &object.List{Elements: []object.Object{}}
	}
	if v, ok := s.given[p]; ok {
		return v
	}
	if p.hasDefault {
		return p.def
	}
	return object.Nil
}
