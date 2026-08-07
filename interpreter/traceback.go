package interpreter

import (
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/token"
)

func stackFrame(module, function string, pos token.Pos) object.Frame {
	frame := object.Frame{Module: module, Function: function, Line: pos.Line, Column: pos.Column}
	if src, ok := pos.Context.(token.Sourcer); ok && src != nil {
		frame.File = src.Source()
	}
	return frame
}

func moduleName(path string) string {
	return source.ModuleName(path)
}

// positionedError tags an error with the position of the statement that
// produced it, so the enclosing function or module boundary can point its
// traceback frame at the failing statement instead of the definition. The tag
// is consumed at that boundary and never escapes it.
type positionedError struct {
	err error
	pos token.Pos
}

func (p *positionedError) Error() string { return p.err.Error() }
func (p *positionedError) Unwrap() error { return p.err }

// positionError tags err with pos. Control-flow signals pass through
// untouched, and an existing tag is kept: the innermost statement wins.
func positionError(err error, pos token.Pos) error {
	switch err.(type) {
	case breakSignal, continueSignal, returnSignal, *positionedError:
		return err
	}
	return &positionedError{err: err, pos: pos}
}

// takePosition strips the position tag, returning the inner error and the
// tagged position, or fallback when err carries no tag.
func takePosition(err error, fallback token.Pos) (error, token.Pos) {
	if p, ok := err.(*positionedError); ok {
		return p.err, p.pos
	}
	return err, fallback
}
