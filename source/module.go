package source

import (
	"path/filepath"
	"strings"
)

// ModuleName derives the module display name from a source file path,
// e.g. "/path/to/mymod.goblin" → "mymod". Both backends use it for
// object.Module.Name and traceback frame module tags, so the two always agree.
func ModuleName(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// IsPathImport reports whether an import path refers to a .goblin file on
// disk (relative or slash-separated) rather than a built-in stdlib module.
func IsPathImport(path string) bool {
	return strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || strings.Contains(path, "/")
}
