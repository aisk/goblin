package object

import "fmt"

type Args []Object

// BindArguments binds positional arguments to parameter names.
func BindArguments(funcName string, params []string, args Args) (map[string]Object, error) {
	if len(args) != len(params) {
		return nil, fmt.Errorf("%s() takes %d positional arguments, got %d", funcName, len(params), len(args))
	}

	bound := make(map[string]Object, len(params))
	for i, param := range params {
		bound[param] = args[i]
	}
	return bound, nil
}
