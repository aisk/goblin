package cli

import (
	"strconv"
	"strings"

	"github.com/aisk/goblin/object"
)

type kind int

const (
	kindArg kind = iota
	kindOption
	kindFlag
)

// param is one declared parameter: a positional Arg, an Option or a Flag.
// The values cli.Arg, cli.Option and cli.Flag return are params without a
// name; the declaration gives each its name.
type param struct {
	object.OpaqueBase
	kind       kind
	name       string // the identifier: the Args attribute
	help       *string
	short      byte
	convert    *object.Function
	choices    []object.Object
	def        object.Object
	hasDefault bool
	many       bool
	required   bool
}

func (p *param) typeName() string {
	switch p.kind {
	case kindArg:
		return "Arg"
	case kindOption:
		return "Option"
	}
	return "Flag"
}

func (p *param) Constructor() *object.Function {
	switch p.kind {
	case kindArg:
		return argType.Function
	case kindOption:
		return optionType.Function
	}
	return flagType.Function
}

func (p *param) String() string            { return "<" + p.typeName() + ">" }
func (p *param) ToString() (string, error) { return p.String(), nil }
func (p *param) GetAttr(name string) (object.Object, error) {
	return nil, object.NewAttributeError("%s has no attribute '%s'", p.typeName(), name)
}
func (p *param) Attributes() []string { return nil }

// named is a copy of p under a name, so one description can be used under
// several names.
func (p *param) named(name string) *param {
	c := *p
	c.name = name
	return &c
}

// long is the parameter's name as typed: --dry-run, <dry-run>.
func (p *param) long() string { return strings.ReplaceAll(p.name, "_", "-") }

// label names the parameter in messages: --output or <target>.
func (p *param) label() string {
	if p.kind == kindArg {
		return "<" + p.long() + ">"
	}
	return "--" + p.long()
}

// command is a declaration: the program's own, or one subcommand's.
type command struct {
	object.OpaqueBase
	name     string
	help     *string
	args     []*param
	options  []*param
	commands []*command
	// raw holds a Command value's declaration until parse reads it.
	rawParams  object.Args
	rawOptions object.Kwargs
}

func (c *command) Constructor() *object.Function { return commandType.Function }
func (c *command) String() string                { return "<Command>" }
func (c *command) ToString() (string, error)     { return c.String(), nil }
func (c *command) GetAttr(name string) (object.Object, error) {
	return nil, object.NewAttributeError("Command has no attribute '%s'", name)
}
func (c *command) Attributes() []string { return nil }

func (c *command) long() string { return strings.ReplaceAll(c.name, "_", "-") }

var (
	_ object.Object = (*param)(nil)
	_ object.Object = (*command)(nil)
)

// helpArg reads the help keyword shared by every description.
func helpArg(p *object.ArgParser) *string {
	help, ok := p.NullableStr("help")
	if !ok {
		return nil
	}
	text := string(help)
	return &text
}

// shortOf reads `short`: nil or one ASCII letter.
func shortOf(fn string, v object.Object) (byte, error) {
	if v == nil || v == object.Nil {
		return 0, nil
	}
	short, ok := v.(object.String)
	if !ok {
		return 0, object.NewTypeError("%s() argument 'short' must be String or Nil, got %s", fn, v.TypeName())
	}
	if len(short) != 1 || !(short[0] >= 'a' && short[0] <= 'z' || short[0] >= 'A' && short[0] <= 'Z') {
		return 0, object.NewValueError("%s() argument 'short' must be one letter, got %s", fn, strconv.Quote(string(short)))
	}
	return short[0], nil
}

// choicesArg reads `choices`: nil or a non-empty List.
func choicesArg(fn string, v object.Object) ([]object.Object, error) {
	if v == nil || v == object.Nil {
		return nil, nil
	}
	list, ok := v.(*object.List)
	if !ok {
		return nil, object.NewTypeError("%s() argument 'choices' must be a non-empty List", fn)
	}
	if len(list.Elements) == 0 {
		return nil, object.NewValueError("%s() argument 'choices' must be a non-empty List", fn)
	}
	return append([]object.Object(nil), list.Elements...), nil
}

// manyDefault checks that a many parameter's default is a List.
func manyDefault(fn string, p *param) error {
	if p.many && p.hasDefault {
		if _, ok := p.def.(*object.List); !ok {
			return object.NewValueError("%s() argument 'default' must be a List when many is true, got %s", fn, p.def.TypeName())
		}
	}
	return nil
}

// Arg(name, help=nil, convert=nil, choices=nil, default=…, many=false)
func newArg(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("Arg", args)
	name := ap.Str("name")
	help := helpArg(ap)
	convert, _ := ap.NullableFunc("convert")
	choices, _ := ap.OptionalAny("choices")
	def, hasDefault := ap.OptionalAny("default")
	many := ap.BoolOr("many", false)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	p := &param{OpaqueBase: object.MakeOpaqueBase("Arg"), kind: kindArg, name: string(name), help: help,
		convert: convert, def: def, hasDefault: hasDefault, many: bool(many)}
	var err error
	if p.choices, err = choicesArg("Arg", choices); err != nil {
		return nil, err
	}
	if err := manyDefault("Arg", p); err != nil {
		return nil, err
	}
	return p, nil
}

// Option(help=nil, short=nil, convert=nil, choices=nil, default=nil, required=false, many=false)
func newOption(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("Option", args)
	help := helpArg(ap)
	shortValue, _ := ap.OptionalAny("short")
	convert, _ := ap.NullableFunc("convert")
	choices, _ := ap.OptionalAny("choices")
	def, _ := ap.OptionalAny("default")
	required := ap.BoolOr("required", false)
	many := ap.BoolOr("many", false)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	short, err := shortOf("Option", shortValue)
	if err != nil {
		return nil, err
	}
	p := &param{OpaqueBase: object.MakeOpaqueBase("Option"), kind: kindOption, help: help, short: short,
		convert: convert, many: bool(many), required: bool(required)}
	if def != nil && def != object.Nil {
		p.def, p.hasDefault = def, true
	}
	if p.choices, err = choicesArg("Option", choices); err != nil {
		return nil, err
	}
	return p, checkOption(p)
}

// checkOption applies the rules of an Option, however it was written.
func checkOption(p *param) error {
	if p.required && p.hasDefault {
		return object.NewValueError("Option() cannot be both required and have a default")
	}
	if _, ok := p.def.(object.Bool); ok && p.hasDefault {
		return object.NewValueError("Option() argument 'default' cannot be a Bool; use cli.Flag()")
	}
	return manyDefault("Option", p)
}

// Flag(help=nil, short=nil, many=false)
func newFlag(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("Flag", args)
	help := helpArg(ap)
	shortValue, _ := ap.OptionalAny("short")
	many := ap.BoolOr("many", false)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	short, err := shortOf("Flag", shortValue)
	if err != nil {
		return nil, err
	}
	return &param{OpaqueBase: object.MakeOpaqueBase("Flag"), kind: kindFlag, help: help, short: short, many: bool(many)}, nil
}

// Command(*params, help=nil, **options): kept as written until parse reads
// it, so its mistakes are reported by parse like any declaration's.
func newCommand(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("Command", args)
	params := ap.Rest()
	help := helpArg(ap)
	options := ap.KeywordRest()
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	for _, o := range options {
		if _, ok := o.Value.(*command); ok {
			return nil, object.NewValueError("Command() commands cannot be nested")
		}
	}
	return &command{OpaqueBase: object.MakeOpaqueBase("Command"), help: help,
		rawParams: append(object.Args(nil), params...), rawOptions: append(object.Kwargs(nil), options...)}, nil
}

// validName matches [a-z][a-z0-9]*(_[a-z0-9]+)*: lower-case words of
// letters and digits joined by single underscores, starting with a letter.
func validName(name string) bool {
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for _, word := range strings.Split(name, "_") {
		if word == "" {
			return false
		}
		for i := 0; i < len(word); i++ {
			if c := word[i]; !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
				return false
			}
		}
	}
	return true
}

func checkName(name string) error {
	if !validName(name) {
		return object.NewValueError("parse() invalid name '%s' (use lower case letters, digits and '_')", name)
	}
	return nil
}

// declare turns parse's arguments into the program's declaration, checking
// every rule of a declaration before the command line is looked at.
func declare(params object.Args, options object.Kwargs, help *string) (*command, error) {
	root, err := declareCommand(params, options, help, true)
	if err != nil {
		return nil, err
	}
	if len(root.commands) > 0 && len(root.args) > 0 {
		return nil, object.NewValueError("parse() cannot combine arguments with commands")
	}
	if err := checkShorts(root.options, nil); err != nil {
		return nil, err
	}
	for _, c := range root.commands {
		for _, p := range append(append([]*param(nil), c.args...), c.options...) {
			if p.name == "command" || findParam(root.options, p.name) != nil {
				return nil, object.NewValueError("parse() command '%s' reuses the name '%s'", c.name, p.name)
			}
		}
		if err := checkShorts(root.options, c.options); err != nil {
			return nil, err
		}
	}
	return root, nil
}

func findParam(params []*param, name string) *param {
	for _, p := range params {
		if p.name == name {
			return p
		}
	}
	return nil
}

// checkShorts rejects a short letter used twice among the options accepted
// together.
func checkShorts(global, own []*param) error {
	seen := map[byte]*param{}
	for _, p := range append(append([]*param(nil), global...), own...) {
		if p.short == 0 {
			continue
		}
		if first, ok := seen[p.short]; ok {
			return object.NewValueError("parse() short option -%c is used by '%s' and '%s'", p.short, first.name, p.name)
		}
		seen[p.short] = p
	}
	return nil
}

func declareCommand(params object.Args, options object.Kwargs, help *string, top bool) (*command, error) {
	c := &command{OpaqueBase: object.MakeOpaqueBase("Command"), help: help}
	names := map[string]bool{}
	use := func(name string) error {
		if err := checkName(name); err != nil {
			return err
		}
		if names[name] {
			return object.NewValueError("parse() duplicate name '%s'", name)
		}
		names[name] = true
		return nil
	}
	for i, v := range params {
		var p *param
		switch value := v.(type) {
		case object.String:
			p = &param{OpaqueBase: object.MakeOpaqueBase("Arg"), kind: kindArg, name: string(value)}
		case *param:
			if value.kind != kindArg {
				return nil, object.NewTypeError("parse() argument %d must be String or Arg, got %s", i+1, value.typeName())
			}
			p = value.named(value.name)
		default:
			return nil, object.NewTypeError("parse() argument %d must be String or Arg, got %s", i+1, v.TypeName())
		}
		if err := use(p.name); err != nil {
			return nil, err
		}
		c.args = append(c.args, p)
	}
	// Order rules: optional Args after required ones; a many Arg last.
	for i, p := range c.args {
		if p.many && i != len(c.args)-1 {
			return nil, object.NewValueError("parse() argument '%s' takes all remaining arguments and must be last", p.name)
		}
		if i > 0 && !p.hasDefault && c.args[i-1].hasDefault {
			return nil, object.NewValueError("parse() required argument '%s' follows optional argument '%s'", p.name, c.args[i-1].name)
		}
	}
	for _, o := range options {
		if err := use(o.Name); err != nil {
			return nil, err
		}
		switch value := o.Value.(type) {
		case object.Bool:
			if value {
				return nil, object.NewValueError("parse() option '%s' cannot default to true; declare the opposite flag (no_%s=false)", o.Name, o.Name)
			}
			c.options = append(c.options, &param{OpaqueBase: object.MakeOpaqueBase("Flag"), kind: kindFlag, name: o.Name})
		case object.String, object.Integer, object.Float:
			c.options = append(c.options, &param{OpaqueBase: object.MakeOpaqueBase("Option"), kind: kindOption, name: o.Name, def: value, hasDefault: true})
		case *object.List:
			c.options = append(c.options, &param{OpaqueBase: object.MakeOpaqueBase("Option"), kind: kindOption, name: o.Name, def: value, hasDefault: true, many: true})
		case *param:
			if value.kind == kindArg {
				return nil, optionKindError(o)
			}
			c.options = append(c.options, value.named(o.Name))
		case *command:
			if !top {
				return nil, object.NewValueError("Command() commands cannot be nested")
			}
			sub, err := declareCommand(value.rawParams, value.rawOptions, value.help, false)
			if err != nil {
				return nil, err
			}
			sub.name = o.Name
			c.commands = append(c.commands, sub)
		default:
			if o.Value == object.Nil {
				c.options = append(c.options, &param{OpaqueBase: object.MakeOpaqueBase("Option"), kind: kindOption, name: o.Name})
				continue
			}
			return nil, optionKindError(o)
		}
	}
	if top && len(c.commands) > 0 && names["command"] {
		return nil, object.NewValueError("parse() duplicate name 'command'")
	}
	return c, nil
}

func optionKindError(o object.KwArg) error {
	return object.NewTypeError("parse() option '%s' must be false, nil, String, Int, Float, List, Flag, Option or Command, got %s", o.Name, o.Value.TypeName())
}
