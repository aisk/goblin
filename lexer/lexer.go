// Code generated by gocc; DO NOT EDIT.

package lexer

import (
	"os"
	"unicode/utf8"

	"github.com/aisk/goblin/token"
)

const (
	NoState    = -1
	NumStates  = 56
	NumSymbols = 63
)

type Lexer struct {
	src     []byte
	pos     int
	line    int
	column  int
	Context token.Context
}

func NewLexer(src []byte) *Lexer {
	lexer := &Lexer{
		src:     src,
		pos:     0,
		line:    1,
		column:  1,
		Context: nil,
	}
	return lexer
}

// SourceContext is a simple instance of a token.Context which
// contains the name of the source file.
type SourceContext struct {
	Filepath string
}

func (s *SourceContext) Source() string {
	return s.Filepath
}

func NewLexerFile(fpath string) (*Lexer, error) {
	src, err := os.ReadFile(fpath)
	if err != nil {
		return nil, err
	}
	lexer := NewLexer(src)
	lexer.Context = &SourceContext{Filepath: fpath}
	return lexer, nil
}

func (l *Lexer) Scan() (tok *token.Token) {
	tok = &token.Token{}
	if l.pos >= len(l.src) {
		tok.Type = token.EOF
		tok.Pos.Offset, tok.Pos.Line, tok.Pos.Column = l.pos, l.line, l.column
		tok.Pos.Context = l.Context
		return
	}
	start, startLine, startColumn, end := l.pos, l.line, l.column, 0
	tok.Type = token.INVALID
	state, rune1, size := 0, rune(-1), 0
	for state != -1 {
		if l.pos >= len(l.src) {
			rune1 = -1
		} else {
			rune1, size = utf8.DecodeRune(l.src[l.pos:])
			l.pos += size
		}

		nextState := -1
		if rune1 != -1 {
			nextState = TransTab[state](rune1)
		}
		state = nextState

		if state != -1 {

			switch rune1 {
			case '\n':
				l.line++
				l.column = 1
			case '\r':
				l.column = 1
			case '\t':
				l.column += 4
			default:
				l.column++
			}

			switch {
			case ActTab[state].Accept != -1:
				tok.Type = ActTab[state].Accept
				end = l.pos
			case ActTab[state].Ignore != "":
				start, startLine, startColumn = l.pos, l.line, l.column
				state = 0
				if start >= len(l.src) {
					tok.Type = token.EOF
				}

			}
		} else {
			if tok.Type == token.INVALID {
				end = l.pos
			}
		}
	}
	if end > start {
		l.pos = end
		tok.Lit = l.src[start:end]
	} else {
		tok.Lit = []byte{}
	}
	tok.Pos.Offset, tok.Pos.Line, tok.Pos.Column = start, startLine, startColumn
	tok.Pos.Context = l.Context

	return
}

func (l *Lexer) Reset() {
	l.pos = 0
}

/*
Lexer symbols:
0: '"'
1: '"'
2: 't'
3: 'r'
4: 'u'
5: 'e'
6: 'f'
7: 'a'
8: 'l'
9: 's'
10: 'e'
11: '('
12: ')'
13: ','
14: 'v'
15: 'a'
16: 'r'
17: '='
18: '{'
19: '}'
20: 'i'
21: 'f'
22: 'e'
23: 'l'
24: 's'
25: 'e'
26: 'w'
27: 'h'
28: 'i'
29: 'l'
30: 'e'
31: 'f'
32: 'u'
33: 'n'
34: 'c'
35: 'r'
36: 'e'
37: 't'
38: 'u'
39: 'r'
40: 'n'
41: '_'
42: '\'
43: '"'
44: '\'
45: ' '
46: '\t'
47: '\n'
48: '\r'
49: '/'
50: '*'
51: '*'
52: '*'
53: '/'
54: '0'-'9'
55: 'a'-'z'
56: 'A'-'Z'
57: \u0001-'!'
58: '#'-'['
59: ']'-\u007f
60: \u0080-\ufffc
61: \ufffe-\U0010ffff
62: .
*/
