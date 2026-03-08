package object

import (
	"fmt"
	"sort"
)

type Args []Object
type Kwargs map[string]Object

type CallArgs struct {
	Positional Args
	Keyword    Kwargs
}

func (c CallArgs) keywordOrEmpty() Kwargs {
	if c.Keyword == nil {
		return Kwargs{}
	}
	return c.Keyword
}

func RequireNoKeyword(funcName string, call CallArgs) error {
	if len(call.Keyword) == 0 {
		return nil
	}
	return fmt.Errorf("%s() does not accept keyword arguments", funcName)
}

// BindArguments binds positional and keyword arguments to parameter names.
// variadicParam and kwVariadicParam are optional capture parameter names.
func BindArguments(funcName string, params []string, variadicParam, kwVariadicParam string, call CallArgs) (map[string]Object, error) {
	if variadicParam == "" && len(call.Positional) > len(params) {
		return nil, fmt.Errorf("%s() takes %d positional arguments, got %d", funcName, len(params), len(call.Positional))
	}

	bound := make(map[string]Object, len(params)+2)
	index := make(map[string]int, len(params))
	for i, param := range params {
		index[param] = i
	}

	fixedCount := len(call.Positional)
	if fixedCount > len(params) {
		fixedCount = len(params)
	}

	for i := 0; i < fixedCount; i++ {
		bound[params[i]] = call.Positional[i]
	}

	kwExtras := make(map[string]Object)
	for key, value := range call.keywordOrEmpty() {
		if _, ok := index[key]; ok {
			if _, exists := bound[key]; exists {
				return nil, fmt.Errorf("%s() got multiple values for argument '%s'", funcName, key)
			}
			bound[key] = value
			continue
		}
		if kwVariadicParam == "" {
			return nil, fmt.Errorf("%s() got an unexpected keyword argument '%s'", funcName, key)
		}
		kwExtras[key] = value
	}

	for _, param := range params {
		if _, ok := bound[param]; !ok {
			return nil, fmt.Errorf("%s() missing required positional argument: '%s'", funcName, param)
		}
	}

	if variadicParam != "" {
		rest := []Object{}
		if len(call.Positional) > len(params) {
			rest = append(rest, call.Positional[len(params):]...)
		}
		bound[variadicParam] = &List{Elements: rest}
	}

	if kwVariadicParam != "" {
		keys := make([]string, 0, len(kwExtras))
		for key := range kwExtras {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		d := NewDict()
		for _, key := range keys {
			d.Set(String(key), kwExtras[key])
		}
		bound[kwVariadicParam] = d
	}

	return bound, nil
}
