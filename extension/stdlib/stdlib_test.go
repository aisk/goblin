package stdlib

import (
	"reflect"
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
