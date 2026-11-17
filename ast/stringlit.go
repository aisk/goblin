package ast

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/token"
)

// SyntaxError is a reduce-action error that knows its exact position. The
// parser reports any reduce error at the token after the construct; source
// replaces that position with Pos, so an error inside a long string literal
// points at the offending character.
type SyntaxError struct {
	Pos     token.Pos
	Message string
}

func (e *SyntaxError) Error() string { return e.Message }

// NewStringLiteral reads a double-quoted string literal. A literal without
// `{` parts is a plain String Literal. One with parts becomes an
// Interpolation whose expression parts are InterpolationSource nodes, which
// source resolves into parsed expressions before anything else sees the tree.
func NewStringLiteral(x any) (any, error) {
	tok := tokenOf(x)
	raw := string(tok.Lit)
	cur := cursor{pos: tok.Pos}
	cur.advance(raw[:1]) // the opening quote
	body := raw[1 : len(raw)-1]

	var parts []Expression
	var text strings.Builder
	textPos := cur.pos
	flush := func() {
		if text.Len() > 0 {
			parts = append(parts, &Literal{expressionMixin: exprAt(textPos), Value: object.String(text.String())})
			text.Reset()
		}
	}
	for i := 0; i < len(body); {
		switch body[i] {
		case '\\':
			n, err := unescape(&text, body[i:])
			if err != nil {
				return nil, &SyntaxError{Pos: cur.pos, Message: err.Error()}
			}
			cur.advance(body[i : i+n])
			i += n
		case '{':
			end, err := interpolationEnd(body, i)
			if err != nil {
				return nil, &SyntaxError{Pos: cur.pos, Message: err.Error()}
			}
			flush()
			cur.advance(body[i : i+1])
			parts = append(parts, &InterpolationSource{expressionMixin: exprAt(cur.pos), Source: body[i+1 : end]})
			cur.advance(body[i+1 : end+1])
			i = end + 1
			textPos = cur.pos
		default:
			_, size := utf8.DecodeRuneInString(body[i:])
			text.WriteString(body[i : i+size])
			cur.advance(body[i : i+size])
			i += size
		}
	}
	if len(parts) == 0 {
		return &Literal{expressionMixin: exprAt(tok.Pos), Value: object.String(text.String())}, nil
	}
	flush()
	return &Interpolation{expressionMixin: exprAt(tok.Pos), Parts: parts}, nil
}

// NewRawStringLiteral reads a backtick string: its text as written, with no
// escapes and no interpolation. Carriage returns are dropped, as in Go, so a
// raw string spanning lines means the same with CRLF line endings.
func NewRawStringLiteral(x any) (any, error) {
	s := lit(x)
	s = strings.ReplaceAll(s[1:len(s)-1], "\r", "")
	return &Literal{expressionMixin: exprAt(tokenOf(x).Pos), Value: object.String(s)}, nil
}

// interpolationEnd returns the index of the `}` closing the interpolation
// that opens at body[open], counting (), [] and {} pairs. The expression
// cannot contain a backslash or a newline (the lexer already ends the
// literal at a `"`), must not be empty, and has no format specifier: a `:`
// outside any bracket is an error, kept free for one. A backtick string
// inside it is skipped whole.
func interpolationEnd(body string, open int) (int, error) {
	depth := 0
	for i := open + 1; i < len(body); i++ {
		switch body[i] {
		case '\\':
			return 0, fmt.Errorf("an interpolation cannot contain a backslash; assign the value to a variable first")
		case '\n':
			return 0, fmt.Errorf("an interpolation cannot span lines; write \\{ for a literal brace")
		case '`':
			closing := strings.IndexByte(body[i+1:], '`')
			if closing < 0 {
				return 0, fmt.Errorf("unterminated interpolation; write \\{ for a literal brace")
			}
			i += closing + 1
		case '(', '[', '{':
			depth++
		case ')', ']':
			depth--
		case '}':
			if depth == 0 {
				if strings.TrimSpace(body[open+1:i]) == "" {
					return 0, fmt.Errorf("empty interpolation; write \\{ for a literal brace")
				}
				return i, nil
			}
			depth--
		case ':':
			if depth == 0 {
				return 0, fmt.Errorf("format specifiers are not supported")
			}
		}
	}
	return 0, fmt.Errorf("unterminated interpolation; write \\{ for a literal brace")
}

// unescape decodes the escape sequence at the start of s into b and returns
// its length. The lexer only lets through the escapes below, so only the
// value of a \u{...} escape can still be wrong.
func unescape(b *strings.Builder, s string) (int, error) {
	switch s[1] {
	case 'n':
		b.WriteByte('\n')
	case 't':
		b.WriteByte('\t')
	case 'r':
		b.WriteByte('\r')
	case '0':
		b.WriteByte(0)
	case 'x':
		v, _ := strconv.ParseUint(s[2:4], 16, 8)
		b.WriteRune(rune(v))
		return 4, nil
	case 'u':
		closing := strings.IndexByte(s, '}')
		digits := s[3:closing]
		v, err := strconv.ParseUint(digits, 16, 32)
		if err != nil || len(digits) > 6 || !utf8.ValidRune(rune(v)) {
			return 0, fmt.Errorf("invalid Unicode escape \\u{%s}: not a Unicode scalar value", digits)
		}
		b.WriteRune(rune(v))
		return closing + 1, nil
	default: // '"', '\\', '{' and '}'
		b.WriteByte(s[1])
	}
	return 2, nil
}

// cursor follows a position through literal text the way the lexer counts
// it: columns in runes, a tab four columns, a carriage return back to the
// first column.
type cursor struct {
	pos token.Pos
}

func (c *cursor) advance(s string) {
	for _, r := range s {
		switch r {
		case '\n':
			c.pos.Line++
			c.pos.Column = 1
		case '\r':
			c.pos.Column = 1
		case '\t':
			c.pos.Column += 4
		default:
			c.pos.Column++
		}
	}
	c.pos.Offset += len(s)
}
