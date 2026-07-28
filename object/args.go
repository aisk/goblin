package object

import (
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

// Scope is anything argument binding can write bindings into, letting callers
// skip the intermediate map that BindArguments returns.
type Scope interface {
	Define(name string, v Object)
}

// ParamDefault computes a parameter's default value. Binding calls it only
// when the argument is absent, so defaults are evaluated per call and a failing
// default expression fails the call itself.
type ParamDefault func() (Object, error)

// BindArgumentsInto binds call arguments straight into scope. For the common
// call shape (fixed parameters, all positional) nothing is allocated at all;
// other shapes fall back to BindArguments.
func BindArgumentsInto(funcName string, params []string, defaults []ParamDefault, varArgsParam, kwArgsParam string, call CallArgs, scope Scope) error {
	if varArgsParam == "" && kwArgsParam == "" && len(call.Keyword) == 0 && len(call.Positional) == len(params) {
		for i, param := range params {
			scope.Define(param, call.Positional[i])
		}
		return nil
	}

	bound, err := BindArguments(funcName, params, defaults, varArgsParam, kwArgsParam, call)
	if err != nil {
		return err
	}
	for name, value := range bound {
		scope.Define(name, value)
	}
	return nil
}

func RequireNoKeyword(funcName string, call CallArgs) error {
	if len(call.Keyword) == 0 {
		return nil
	}
	return NewTypeError("%s() does not accept keyword arguments", funcName)
}

// BindArguments binds positional and keyword arguments to parameter names.
// defaults is either nil or parallel to params, with a nil entry per required
// parameter. varArgsParam and kwArgsParam are optional capture parameter names.
func BindArguments(funcName string, params []string, defaults []ParamDefault, varArgsParam, kwArgsParam string, call CallArgs) (map[string]Object, error) {
	if varArgsParam == "" && len(call.Positional) > len(params) {
		return nil, NewTypeError("%s() takes %d positional arguments, got %d", funcName, len(params), len(call.Positional))
	}

	// Fast path for the overwhelmingly common call shape: only fixed
	// parameters, all supplied positionally. Skips the index and kwExtras
	// maps, which the general path below allocates on every single call.
	if varArgsParam == "" && kwArgsParam == "" && len(call.Keyword) == 0 && len(call.Positional) == len(params) {
		bound := make(map[string]Object, len(params))
		for i, param := range params {
			bound[param] = call.Positional[i]
		}
		return bound, nil
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
				return nil, NewTypeError("%s() got multiple values for argument '%s'", funcName, key)
			}
			bound[key] = value
			continue
		}
		if kwArgsParam == "" {
			return nil, NewTypeError("%s() got an unexpected keyword argument '%s'", funcName, key)
		}
		kwExtras[key] = value
	}

	for i, param := range params {
		if _, ok := bound[param]; ok {
			continue
		}
		if i < len(defaults) && defaults[i] != nil {
			value, err := defaults[i]()
			if err != nil {
				return nil, err
			}
			bound[param] = value
			continue
		}
		return nil, NewTypeError("%s() missing required positional argument: '%s'", funcName, param)
	}

	if varArgsParam != "" {
		rest := []Object{}
		if len(call.Positional) > len(params) {
			rest = append(rest, call.Positional[len(params):]...)
		}
		bound[varArgsParam] = &List{Elements: rest}
	}

	if kwArgsParam != "" {
		keys := make([]string, 0, len(kwExtras))
		for key := range kwExtras {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		d := NewDict()
		for _, key := range keys {
			if err := d.Set(String(key), kwExtras[key]); err != nil {
				return nil, err
			}
		}
		bound[kwArgsParam] = d
	}

	return bound, nil
}
