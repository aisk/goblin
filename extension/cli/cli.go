// Package cli is the cli module: a script describes its command line the
// way a call is written (positional names, then name=default) and gets back
// an Args value with one attribute per name.
package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aisk/goblin/extension/internal/procargs"
	"github.com/aisk/goblin/object"
)

var (
	// UsageError is raised when a command line given as a List does not
	// fit the declaration.
	UsageError = object.NewErrorKind("UsageError", object.BaseError)
	// HelpRequested is raised when a command line given as a List asks for
	// --help; its message is the help text.
	HelpRequested = object.NewErrorKind("HelpRequested", object.BaseError)

	argType     = object.NewNativeConstructor("Arg", newArg)
	optionType  = object.NewNativeConstructor("Option", newOption)
	flagType    = object.NewNativeConstructor("Flag", newFlag)
	commandType = object.NewNativeConstructor("Command", newCommand)
	argsType    = object.NewOpaqueConstructor("Args", "cli.parse()")
)

// Process mode writes to the streams of the moment, as print does, and
// exits through exit, which tests replace.
var (
	exit   = os.Exit
	stdout = func() io.Writer { return os.Stdout }
	stderr = func() io.Writer { return os.Stderr }
)

// Execute builds the cli module over the process's own command line, as a
// compiled program sees it.
func Execute() (object.Object, error) {
	return ExecuteWithFrozenArgs(procargs.Process())
}

// ExecuteWithFrozenArgs builds the cli module over a given command line,
// the program followed by its arguments: the interpreter passes the
// script's.
func ExecuteWithFrozenArgs(argv []string) (object.Object, error) {
	program, args := procargs.Split(argv)
	name := filepath.Base(program)
	if program == "" {
		name = "program"
	}
	process := &object.List{Elements: make([]object.Object, len(args))}
	for i, a := range args {
		process.Elements[i] = object.String(a)
	}
	parse := func(call object.CallArgs) (object.Object, error) {
		return parseCall(name, process, call)
	}
	return object.NewModule("cli", map[string]object.Object{
		"Arg":           argType.Function,
		"Option":        optionType.Function,
		"Flag":          flagType.Function,
		"Command":       commandType.Function,
		"Args":          argsType.Function,
		"UsageError":    UsageError,
		"HelpRequested": HelpRequested,
	}, object.Funcs{
		"parse": parse,
	}), nil
}

// parseCall is cli.parse: read the declaration, then the command line,
// from the process (exiting on bad input or help) or from argv= (raising).
func parseCall(program string, process *object.List, call object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("parse", call)
	params := p.Rest()
	help, hasHelp := p.NullableStr("help")
	argvArg, _ := p.OptionalAny("argv")
	options := p.KeywordRest()
	if err := p.Finish(); err != nil {
		return nil, err
	}
	var argv []string
	given := argvArg != nil && argvArg != object.Nil
	if given {
		list, ok := argvArg.(*object.List)
		if !ok {
			return nil, object.NewTypeError("parse() argument 'argv' must be a List of String, got %s", argvArg.TypeName())
		}
		var err error
		if argv, err = strings_(list); err != nil {
			return nil, err
		}
	} else {
		argv, _ = strings_(process)
	}
	var description *string
	if hasHelp {
		text := string(help)
		description = &text
	}
	root, err := declare(params, options, description)
	if err != nil {
		return nil, err
	}
	result, err := scan(program, root, argv)
	if err == nil {
		return result, nil
	}
	var stop *stopError
	if !errors.As(err, &stop) {
		return nil, err
	}
	if given {
		if stop.help {
			return nil, object.NewKindError(HelpRequested, "%s", stop.text)
		}
		return nil, object.NewKindError(UsageError, "%s", stop.text).With("usage", object.String(stop.usage))
	}
	if stop.help {
		io.WriteString(stdout(), stop.text+"\n")
		exit(0)
		return object.Nil, nil
	}
	io.WriteString(stderr(), "error: "+stop.text+"\n"+stop.usage+"\n"+"Run '"+stop.helpCommand+"' for details.\n")
	exit(2)
	return object.Nil, nil
}

func strings_(list *object.List) ([]string, error) {
	out := make([]string, len(list.Elements))
	for i, e := range list.Elements {
		s, ok := e.(object.String)
		if !ok {
			return nil, object.NewTypeError("parse() argument 'argv' must be a List of String, got List containing %s", e.TypeName())
		}
		out[i] = string(s)
	}
	return out, nil
}

// stopError ends a scan: a usage error or a request for help, with what
// either mode needs to report it.
type stopError struct {
	help        bool
	text        string // the reason, or the help text
	usage       string // the usage line of the command in effect
	helpCommand string // how to ask for its help: "tool.goblin build --help"
}

func (e *stopError) Error() string { return e.text }

// render shows a value as Goblin source would write it ("a", 1, nil).
func render(v object.Object) string {
	text, err := (&object.List{Elements: []object.Object{v}}).ToString()
	if err != nil {
		return v.TypeName()
	}
	return text[1 : len(text)-1]
}

// plain shows a value as interpolation would (a String without quotes).
func plain(v object.Object) string {
	text, err := v.ToString()
	if err != nil {
		return render(v)
	}
	return text
}

// Args is what parse returns: an immutable value with one attribute per
// declared parameter, in declaration order, and no methods.
type Args struct {
	object.OpaqueBase
	names  []string
	values map[string]object.Object
}

func (a *Args) set(name string, v object.Object) {
	if _, ok := a.values[name]; !ok {
		a.names = append(a.names, name)
	}
	a.values[name] = v
}

func (a *Args) Constructor() *object.Function { return argsType.Function }

func (a *Args) String() string {
	parts := make([]string, len(a.names))
	for i, name := range a.names {
		parts[i] = name + "=" + render(a.values[name])
	}
	return "Args(" + strings.Join(parts, ", ") + ")"
}

func (a *Args) ToString() (string, error) { return a.String(), nil }

func (a *Args) GetAttr(name string) (object.Object, error) {
	if v, ok := a.values[name]; ok {
		return v, nil
	}
	return nil, object.NewAttributeError("Args has no attribute '%s'", name)
}

func (a *Args) Attributes() []string { return append([]string(nil), a.names...) }

func (a *Args) SetAttr(string, object.Object) (bool, error) {
	return true, object.NewAttributeError("Args is read-only")
}

var _ object.Object = (*Args)(nil)
