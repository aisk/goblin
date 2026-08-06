package interpreter

import (
	"testing"

	"github.com/aisk/goblin/transpiler"
)

// builtinModules and the transpiler's knownModules must stay in sync: a
// module registered on only one side imports fine in one backend and raises
// ImportError in the other. "os" is the single intentional asymmetry — the
// interpreter binds it per run so argv stays scoped to the script (see the
// builtinModules comment).
func TestBuiltinModulesMatchTranspilerKnownModules(t *testing.T) {
	interpreterNames := map[string]bool{"os": true}
	for name := range builtinModules {
		interpreterNames[name] = true
	}

	for _, name := range transpiler.KnownModuleNames() {
		if !interpreterNames[name] {
			t.Errorf("module %q is known to the transpiler but missing from the interpreter's builtinModules", name)
		}
		delete(interpreterNames, name)
	}
	for name := range interpreterNames {
		t.Errorf("module %q is registered in the interpreter but unknown to the transpiler", name)
	}
}
