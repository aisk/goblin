package stdlib

import (
	"reflect"
	"regexp"
	"runtime"
	"testing"
)

// The transpiler turns a module name into the Go import path
// extension/<name> and calls its Execute, so every registered executor must
// be exactly that function.
func TestExecutorsLiveAtTheirModulePath(t *testing.T) {
	for _, name := range Names() {
		exec, _ := Lookup(name)
		got := runtime.FuncForPC(reflect.ValueOf(exec).Pointer()).Name()
		want := "github.com/aisk/goblin/extension/" + name + ".Execute"
		if got != want {
			t.Errorf("module %q is registered with %s, want %s", name, got, want)
		}
	}
}

// Stdlib modules have flat names: the name is the imported binding, the Go
// package directory under extension/ and, with "_module" appended, the
// generated-code variable, so it must be a plain lowercase word.
var moduleNamePattern = regexp.MustCompile(`^[a-z0-9]+$`)

func TestModuleNamesAreFlat(t *testing.T) {
	for _, name := range Names() {
		if !moduleNamePattern.MatchString(name) {
			t.Errorf("module name %q must be one lowercase word of letters and digits", name)
		}
	}
}
