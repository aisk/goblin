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

// The transpiler spells a module's variable by replacing "/" with "_", which
// is only one to one while names are lowercase segments without "_".
var moduleNamePattern = regexp.MustCompile(`^[a-z0-9]+(/[a-z0-9]+)*$`)

func TestModuleNamesAreSimplePaths(t *testing.T) {
	for _, name := range Names() {
		if !moduleNamePattern.MatchString(name) {
			t.Errorf("module name %q must be lowercase segments of letters and digits", name)
		}
	}
}
