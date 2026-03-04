package object

import "fmt"

type Args []Object
type KwArgs map[string]Object

// BindArguments binds positional and keyword arguments to parameter names.
func BindArguments(funcName string, params []string, args Args, kwargs KwArgs) (map[string]Object, error) {
	if len(args) > len(params) {
		return nil, fmt.Errorf("%s() takes at most %d positional arguments, got %d", funcName, len(params), len(args))
	}

	paramSet := make(map[string]struct{}, len(params))
	bound := make(map[string]Object, len(params))

	for i, param := range params {
		paramSet[param] = struct{}{}
		if i < len(args) {
			bound[param] = args[i]
		}
	}

	for name, value := range kwargs {
		if _, ok := paramSet[name]; !ok {
			return nil, fmt.Errorf("%s() got an unexpected keyword argument '%s'", funcName, name)
		}
		if _, exists := bound[name]; exists {
			return nil, fmt.Errorf("%s() got multiple values for argument '%s'", funcName, name)
		}
		bound[name] = value
	}

	for _, param := range params {
		if _, ok := bound[param]; !ok {
			return nil, fmt.Errorf("%s() missing required argument '%s'", funcName, param)
		}
	}

	return bound, nil
}
