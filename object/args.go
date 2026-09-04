package object

import (
	"sort"
)

type Args []Object

// KwArg is one keyword argument, paired with the name it was supplied under.
type KwArg struct {
	Name  string
	Value Object
}

// Kwargs holds a call's keyword arguments in the order they were supplied. A
// call carries a handful of keywords at most, so a linear scan beats the map
// this used to be: the map had to be allocated on every call that passed even
// one keyword argument, which the argument-parsing path then paid for again.
type Kwargs []KwArg

// Get returns the value supplied under name, reporting whether it was present.
func (k Kwargs) Get(name string) (Object, bool) {
	for i := range k {
		if k[i].Name == name {
			return k[i].Value, true
		}
	}
	return nil, false
}

// Set binds name to value, replacing an existing binding of that name.
func (k *Kwargs) Set(name string, value Object) {
	for i := range *k {
		if (*k)[i].Name == name {
			(*k)[i].Value = value
			return
		}
	}
	*k = append(*k, KwArg{Name: name, Value: value})
}

type CallArgs struct {
	Positional Args
	Keyword    Kwargs
}

// AddKeyword records one keyword argument, rejecting a name that was already
// supplied. Both backends build keyword arguments through this so duplicate
// detection behaves identically.
func (c *CallArgs) AddKeyword(name string, value Object) error {
	if _, exists := c.Keyword.Get(name); exists {
		return NewTypeError("got multiple values for argument '%s'", name)
	}
	c.Keyword = append(c.Keyword, KwArg{Name: name, Value: value})
	return nil
}

// UnpackKeywords merges a **-unpacked value into the keyword arguments. The
// value must be a dict with string keys; a name that was already supplied is
// rejected like any other duplicate keyword.
func (c *CallArgs) UnpackKeywords(v Object) error {
	d, ok := v.(*Dict)
	if !ok {
		return NewTypeError("argument after ** must be a dict, got %s", v.TypeName())
	}
	for _, entry := range d.Entries() {
		key, ok := entry.Key.(String)
		if !ok {
			return NewTypeError("keyword argument name must be a string, got %s", entry.Key.TypeName())
		}
		if err := c.AddKeyword(string(key), entry.Value); err != nil {
			return err
		}
	}
	return nil
}

// copy returns the same arguments backed by fresh storage. Passing arguments
// into a callee escape analysis cannot see through (an interface method, a
// function value) marks them as escaping, which would force every call site's
// literal onto the heap. Copying at the boundary keeps that cost on the paths
// that actually take it.
func (c CallArgs) copy() CallArgs {
	var out CallArgs
	if len(c.Positional) > 0 {
		out.Positional = append(Args(nil), c.Positional...)
	}
	if len(c.Keyword) > 0 {
		out.Keyword = append(Kwargs(nil), c.Keyword...)
	}
	return out
}

// Scope is anything argument binding can write bindings into, letting callers
// skip the intermediate slice that BindArguments returns.
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
	for i, param := range params {
		scope.Define(param, bound[i])
	}
	next := len(params)
	if varArgsParam != "" {
		scope.Define(varArgsParam, bound[next])
		next++
	}
	if kwArgsParam != "" {
		scope.Define(kwArgsParam, bound[next])
	}
	return nil
}

// BindArguments binds positional and keyword arguments to parameters, returning
// one value per parameter in declaration order, followed by the *varargs list
// and the **kwargs dict when those capture parameters are declared. defaults is
// either nil or parallel to params, with a nil entry per required parameter.
func BindArguments(funcName string, params []string, defaults []ParamDefault, varArgsParam, kwArgsParam string, call CallArgs) ([]Object, error) {
	if varArgsParam == "" && len(call.Positional) > len(params) {
		return nil, NewTypeError("%s() takes %d positional arguments, got %d", funcName, len(params), len(call.Positional))
	}

	extra := 0
	if varArgsParam != "" {
		extra++
	}
	if kwArgsParam != "" {
		extra++
	}
	bound := make([]Object, len(params)+extra)

	// Fast path for the overwhelmingly common call shape: only fixed
	// parameters, all supplied positionally.
	if extra == 0 && len(call.Keyword) == 0 && len(call.Positional) == len(params) {
		copy(bound, call.Positional)
		return bound, nil
	}

	fixedCount := len(call.Positional)
	if fixedCount > len(params) {
		fixedCount = len(params)
	}
	copy(bound, call.Positional[:fixedCount])

	var kwExtras Kwargs
	for _, kw := range call.Keyword {
		index := -1
		for i, param := range params {
			if param == kw.Name {
				index = i
				break
			}
		}
		if index >= 0 {
			// Every slot below fixedCount already took a positional
			// argument, so a keyword naming one is a second value for it.
			if index < fixedCount {
				return nil, NewTypeError("%s() got multiple values for argument '%s'", funcName, kw.Name)
			}
			bound[index] = kw.Value
			continue
		}
		if kwArgsParam == "" {
			return nil, NewTypeError("%s() got an unexpected keyword argument '%s'", funcName, kw.Name)
		}
		kwExtras = append(kwExtras, kw)
	}

	for i, param := range params {
		if bound[i] != nil {
			continue
		}
		if i < len(defaults) && defaults[i] != nil {
			value, err := defaults[i]()
			if err != nil {
				return nil, err
			}
			bound[i] = value
			continue
		}
		return nil, NewTypeError("%s() missing required positional argument: '%s'", funcName, param)
	}

	next := len(params)
	if varArgsParam != "" {
		rest := []Object{}
		if len(call.Positional) > len(params) {
			rest = append(rest, call.Positional[len(params):]...)
		}
		bound[next] = &List{Elements: rest}
		next++
	}

	if kwArgsParam != "" {
		sort.Slice(kwExtras, func(i, j int) bool { return kwExtras[i].Name < kwExtras[j].Name })
		d := NewDict()
		for _, kw := range kwExtras {
			if err := d.Set(String(kw.Name), kw.Value); err != nil {
				return nil, err
			}
		}
		bound[next] = d
	}

	return bound, nil
}
