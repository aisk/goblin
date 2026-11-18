package procargs

import (
	"strings"

	"github.com/aisk/goblin/object"
)

// CheckEnv validates an environment variable before it is set, in this
// process (os.env) or for a child (exec's env=), so both refuse the same
// names and values with the same words. A name with "=" would set another
// variable and an empty one or a NUL cannot be represented. prefix starts
// the message: "run() " for a call, "" for index assignment, which is
// syntax.
func CheckEnv(prefix, name, value string) error {
	if name == "" || strings.ContainsAny(name, "=\x00") {
		return object.NewValueError("%sinvalid environment variable name %q", prefix, name)
	}
	if strings.ContainsRune(value, 0) {
		return object.NewValueError("%senvironment variable '%s' value contains NUL", prefix, name)
	}
	return nil
}
