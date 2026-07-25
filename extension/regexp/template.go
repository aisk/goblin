package regexp

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aisk/goblin/object"
)

// checkTemplate rejects a replacement template that references a capture group
// the pattern does not have. Go's Expand silently turns such a reference into
// the empty string, which drops text without a word; every other lookup in this
// module raises, so replace() does too.
//
// The scan mirrors regexp.Regexp.expand: it must accept exactly what Expand
// would resolve. A malformed reference — a '$' that starts no valid name — is
// kept as literal text by Expand and is therefore not an error here.
func (p *Pattern) checkTemplate(template string) error {
	for {
		_, after, ok := strings.Cut(template, "$")
		if !ok {
			return nil
		}
		template = after
		if strings.HasPrefix(template, "$") {
			// "$$" is an escaped literal '$'.
			template = template[1:]
			continue
		}
		name, num, rest, ok := extractRef(template)
		if !ok {
			continue
		}
		template = rest
		if num >= 0 {
			if num >= len(p.names) {
				return object.NewValueError(
					"replace() template references group %d, but the pattern has %d capture group(s)",
					num, len(p.names)-1)
			}
			continue
		}
		if !p.hasGroupName(name) {
			if name[0] >= '0' && name[0] <= '9' {
				return object.NewValueError(
					"replace() template references unknown capture group '%s'; write ${...} to separate a group number from the text after it",
					name)
			}
			return object.NewValueError("replace() template references unknown capture group '%s'", name)
		}
	}
}

// extractRef reads one group reference from the template text following a '$'.
// It is a copy of the unexported regexp.extract from the standard library, and
// must stay in step with it: num is the group number, or -1 when the reference
// is by name; ok is false when the text is not a reference at all.
func extractRef(str string) (name string, num int, rest string, ok bool) {
	if str == "" {
		return
	}
	brace := false
	if str[0] == '{' {
		brace = true
		str = str[1:]
	}
	i := 0
	for i < len(str) {
		r, size := utf8.DecodeRuneInString(str[i:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			break
		}
		i += size
	}
	if i == 0 {
		// An empty name is not a reference.
		return
	}
	name = str[:i]
	if brace {
		if i >= len(str) || str[i] != '}' {
			// Missing closing brace.
			return
		}
		i++
	}
	// Read the name as a group number, rejecting anything Expand would not take
	// as one: non-digits, more than three digits, or a leading zero.
	num = 0
	for j := 0; j < len(name); j++ {
		if name[j] < '0' || '9' < name[j] || len(name) > 3 {
			num = -1
			break
		}
		num = num*10 + int(name[j]) - '0'
	}
	if name[0] == '0' && len(name) > 1 {
		num = -1
	}
	rest = str[i:]
	ok = true
	return
}
