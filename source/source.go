// Package source wraps the generated lexer constructors so every way of
// loading Goblin code goes through the same normalization.
package source

import (
	"os"

	"github.com/aisk/goblin/lexer"
)

// NewLexer wraps lexer.NewLexer, appending a trailing newline when the
// source lacks one. The grammar's comment rule can only terminate on '\n'
// (gocc's generated lexer ends an ignored token at its first accepting
// state, so the rule cannot instead accept at end-of-file), which would
// otherwise make a comment on the last line a lex error.
func NewLexer(src []byte) *lexer.Lexer {
	if len(src) > 0 && src[len(src)-1] != '\n' {
		src = append(src, '\n')
	}
	return lexer.NewLexer(src)
}

// NewLexerFile is lexer.NewLexerFile with the same trailing-newline
// normalization as NewLexer.
func NewLexerFile(path string) (*lexer.Lexer, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	l := NewLexer(src)
	l.Context = &lexer.SourceContext{Filepath: path}
	return l, nil
}
