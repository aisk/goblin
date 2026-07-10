package interpreter

import (
	"path/filepath"
	"strings"

	"github.com/aisk/goblin/object"
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
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
