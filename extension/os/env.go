package os

import (
	"os"
	"sort"
	"strings"

	"github.com/aisk/goblin/extension/internal/procargs"
	"github.com/aisk/goblin/object"
)

var envType = object.NewOpaqueConstructor("Env", "os.env")

// Env is os.env, the process environment, read and changed live. It uses
// Dict's vocabulary without being a Dict.
type Env struct {
	object.OpaqueBase
}

var theEnv = &Env{OpaqueBase: object.MakeOpaqueBase("Env")}

func (e *Env) Constructor() *object.Function { return envType.Function }
func (e *Env) String() string                { return "<Env>" }
func (e *Env) ToString() (string, error)     { return e.String(), nil }

func (e *Env) Equals(other object.Object) (bool, error) { return other == object.Object(e), nil }

func envName(v object.Object) (string, error) {
	s, ok := v.(object.String)
	if !ok {
		return "", object.NewTypeError("environment variable name must be String, got %s", v.TypeName())
	}
	return string(s), nil
}

// Index performs os.env[name].
func (e *Env) Index(key object.Object) (object.Object, error) {
	name, err := envName(key)
	if err != nil {
		return nil, err
	}
	value, ok := os.LookupEnv(name)
	if !ok {
		return nil, object.NewKeyError("environment variable '%s' is not set", name)
	}
	return object.String(value), nil
}

// SetIndex performs os.env[name] = value.
func (e *Env) SetIndex(key, value object.Object) (bool, error) {
	name, err := envName(key)
	if err != nil {
		return true, err
	}
	s, ok := value.(object.String)
	if !ok {
		return true, object.NewTypeError("environment variable value must be String, got %s", value.TypeName())
	}
	if err := procargs.CheckEnv("", name, string(s)); err != nil {
		return true, err
	}
	if err := os.Setenv(name, string(s)); err != nil {
		return true, object.WrapNativeError(object.ValueError, "cannot set environment variable '"+name+"'", err)
	}
	return true, nil
}

func (e *Env) GetAttr(name string) (object.Object, error) {
	switch name {
	case "get":
		return &object.Function{Name: "get", Fn: e.get}, nil
	case "contains":
		return &object.Function{Name: "contains", Fn: e.contains}, nil
	case "remove":
		return &object.Function{Name: "remove", Fn: e.remove}, nil
	case "to_dict":
		return &object.Function{Name: "to_dict", Fn: e.toDict}, nil
	}
	return nil, object.NewAttributeError("Env has no attribute '%s'", name)
}

func (e *Env) Attributes() []string { return []string{"get", "contains", "remove", "to_dict"} }

func (e *Env) get(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("get", args)
	name := p.Str("name")
	def := p.AnyOr("default", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if value, ok := os.LookupEnv(string(name)); ok {
		return object.String(value), nil
	}
	return def, nil
}

func (e *Env) contains(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("contains", args)
	name := p.Str("name")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	_, ok := os.LookupEnv(string(name))
	return object.Bool(ok), nil
}

func (e *Env) remove(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("remove", args)
	name := p.Str("name")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if err := os.Unsetenv(string(name)); err != nil {
		return nil, object.WrapNativeError(object.ValueError, "remove() cannot unset environment variable '"+string(name)+"'", err)
	}
	return e, nil
}

func (e *Env) toDict(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("to_dict", args); err != nil {
		return nil, err
	}
	type variable struct{ name, value string }
	var variables []variable
	for _, entry := range os.Environ() {
		name, value, found := strings.Cut(entry, "=")
		// Windows lists per-drive working directories as "=C:=C:\\".
		if found && name != "" {
			variables = append(variables, variable{name, value})
		}
	}
	sort.SliceStable(variables, func(i, j int) bool { return variables[i].name < variables[j].name })
	result := object.NewDict()
	for _, v := range variables {
		if err := result.Set(object.String(v.name), object.String(v.value)); err != nil {
			return nil, err
		}
	}
	return result, nil
}
