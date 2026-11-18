package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func str(s string) object.Object { return object.String(s) }

func strList(items ...string) *object.List {
	l := &object.List{Elements: []object.Object{}}
	for _, s := range items {
		l.Elements = append(l.Elements, object.String(s))
	}
	return l
}

func kw(name string, v object.Object) object.KwArg { return object.KwArg{Name: name, Value: v} }

func module(t *testing.T, argv ...string) *object.Module {
	t.Helper()
	m, err := ExecuteWithFrozenArgs(argv)
	if err != nil {
		t.Fatal(err)
	}
	return m.(*object.Module)
}

func construct(t *testing.T, name string, pos object.Args, kws ...object.KwArg) object.Object {
	t.Helper()
	fn := module(t, "tool")
	v, err := fn.Members[name].(*object.Function).Call(object.CallArgs{Positional: pos, Keyword: kws})
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return v
}

// parse calls cli.parse with argv given as a List.
func parse(t *testing.T, argv []string, pos object.Args, kws ...object.KwArg) (object.Object, error) {
	t.Helper()
	parseFn := module(t, "/usr/bin/tool").Members["parse"].(*object.Function)
	kws = append(kws, kw("argv", strList(argv...)))
	return parseFn.Call(object.CallArgs{Positional: pos, Keyword: kws})
}

func mustParse(t *testing.T, argv []string, pos object.Args, kws ...object.KwArg) *Args {
	t.Helper()
	v, err := parse(t, argv, pos, kws...)
	if err != nil {
		t.Fatalf("parse(%q) error = %v", argv, err)
	}
	return v.(*Args)
}

func usageError(t *testing.T, err error) (message, usage string) {
	t.Helper()
	var e *object.Error
	if !errors.As(err, &e) || !errors.Is(err, UsageError) {
		t.Fatalf("error = %v, want a UsageError", err)
	}
	u, _ := e.Field("usage")
	return e.Error(), string(u.(object.String))
}

func TestScannerGrammar(t *testing.T) {
	output := kw("output", object.Nil)
	verbose := kw("verbose", construct(t, "Flag", nil, kw("short", str("v")), kw("many", object.Bool(true))))
	short := kw("o", construct(t, "Option", nil))
	cases := []struct {
		argv []string
		want string
	}{
		{[]string{"--output="}, `Args(rest=[], output="", verbose=0, o=nil)`},
		{[]string{"--output", "--"}, ``}, // "--" is not taken as a value
		{[]string{"--output=--"}, `Args(rest=[], output="--", verbose=0, o=nil)`},
		{[]string{"--o", "-"}, `Args(rest=[], output=nil, verbose=0, o="-")`},
		{[]string{"--", "-v", "--output"}, `Args(rest=["-v", "--output"], output=nil, verbose=0, o=nil)`},
		{[]string{"x", "--", "--"}, `Args(rest=["x", "--"], output=nil, verbose=0, o=nil)`},
		{[]string{"-vvv", "-v"}, `Args(rest=[], output=nil, verbose=4, o=nil)`},
		{[]string{"-5", "-.5", "-1.5"}, `Args(rest=["-5", "-.5", "-1.5"], output=nil, verbose=0, o=nil)`},
		{[]string{"--output", "-5"}, `Args(rest=[], output="-5", verbose=0, o=nil)`},
		{[]string{"--o", "a", "--o", "b"}, `Args(rest=[], output=nil, verbose=0, o="b")`},
	}
	rest := construct(t, "Arg", object.Args{str("rest")}, kw("many", object.Bool(true)), kw("default", &object.List{}))
	for _, c := range cases {
		v, err := parse(t, c.argv, object.Args{rest}, output, verbose, short)
		if c.want == "" {
			if msg, _ := usageError(t, err); msg != "option --output needs a value" {
				t.Errorf("%q: %s", c.argv, msg)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", c.argv, err)
			continue
		}
		if got := v.(*Args).String(); got != c.want {
			t.Errorf("%q = %s, want %s", c.argv, got, c.want)
		}
	}
}

func TestClusterEndingInOption(t *testing.T) {
	o := kw("output", construct(t, "Option", nil, kw("short", str("o"))))
	v := kw("verbose", construct(t, "Flag", nil, kw("short", str("v"))))
	_, err := parse(t, []string{"-vo"}, nil, o, v)
	if msg, _ := usageError(t, err); msg != "option --output needs a value" {
		t.Fatal(msg)
	}
	_, err = parse(t, []string{"-vo", "-v"}, nil, o, v)
	if msg, _ := usageError(t, err); msg != "option --output needs a value" {
		t.Fatal(msg)
	}
	if got := mustParse(t, []string{"-vo", "-"}, nil, o, v).String(); got != `Args(output="-", verbose=true)` {
		t.Fatal(got)
	}
	if got := mustParse(t, []string{"-vo="}, nil, o, v).String(); got != `Args(output="", verbose=true)` {
		t.Fatal(got)
	}
}

func TestUsageMessages(t *testing.T) {
	count := kw("count", object.Integer(1))
	width := kw("width", object.Integer(0))
	cases := []struct {
		argv []string
		pos  object.Args
		kws  []object.KwArg
		want string
	}{
		{[]string{"--widht"}, nil, []object.KwArg{width, count}, "unknown option --widht (did you mean --width?)"},
		{[]string{"--cnt"}, nil, []object.KwArg{width, count}, "unknown option --cnt (did you mean --count?)"},
		{[]string{"--verb"}, nil, []object.KwArg{width}, "unknown option --verb"},
		{[]string{"--Count"}, nil, []object.KwArg{count}, "unknown option --Count (did you mean --count?)"},
		{[]string{"--count", "x"}, nil, []object.KwArg{count}, `invalid value "x" for --count: expected an integer`},
		{[]string{"a", "b"}, object.Args{str("one")}, nil, `unexpected argument "b"`},
		{[]string{}, object.Args{str("one"), str("two_words")}, nil, "missing arguments <one> <two-words>"},
	}
	for _, c := range cases {
		_, err := parse(t, c.argv, c.pos, c.kws...)
		if msg, _ := usageError(t, err); msg != c.want {
			t.Errorf("%q: %s, want %s", c.argv, msg, c.want)
		}
	}
}

func TestNearest(t *testing.T) {
	cases := []struct {
		word       string
		candidates []string
		want       string
		ok         bool
	}{
		{"widht", []string{"width", "height"}, "width", true},
		{"ab", []string{"xy", "ac", "ad"}, "ac", true}, // a tie goes to the earlier
		{"abc", []string{"xyz"}, "", false},
		{"build", nil, "", false},
		{"tst", []string{"test"}, "test", true},
	}
	for _, c := range cases {
		got, ok := nearest(c.word, c.candidates)
		if got != c.want && c.ok || ok != c.ok {
			t.Errorf("nearest(%q) = %q, %v", c.word, got, ok)
		}
	}
	if d := editDistance("kitten", "sitting"); d != 3 {
		t.Errorf("editDistance = %d", d)
	}
}

func help(t *testing.T, argv []string, pos object.Args, kws ...object.KwArg) string {
	t.Helper()
	_, err := parse(t, argv, pos, kws...)
	if !errors.Is(err, HelpRequested) {
		t.Fatalf("error = %v, want HelpRequested", err)
	}
	return err.Error()
}

func TestHelpLayout(t *testing.T) {
	resize := []object.KwArg{
		kw("width", construct(t, "Option", object.Args{str("target width in pixels")}, kw("short", str("w")), kw("default", object.Integer(800)))),
		kw("format", construct(t, "Option", nil, kw("choices", strList("png", "jpg")), kw("default", str("png")))),
		kw("output", construct(t, "Option", object.Args{str("directory to write to")}, kw("short", str("o")), kw("required", object.Bool(true)))),
		kw("dry_run", object.Bool(false)),
		kw("verbose", construct(t, "Flag", object.Args{str("say what is being done")}, kw("short", str("v")), kw("many", object.Bool(true)))),
		kw("help", str("Resize images into a directory.")),
	}
	files := construct(t, "Arg", object.Args{str("files"), str("images to resize")}, kw("many", object.Bool(true)))
	want := `usage: tool [options] <files>...

Resize images into a directory.

arguments:
  <files>...              images to resize

options:
  -w, --width <width>     target width in pixels (default: 800)
      --format <png|jpg>  (default: png)
  -o, --output <output>   directory to write to (required)
      --dry-run
  -v, --verbose           say what is being done
  -h, --help              show this help and exit`
	if got := help(t, []string{"--help"}, object.Args{files}, resize...); got != want {
		t.Errorf("help:\n%s\nwant:\n%s", got, want)
	}

	// Help is reached even with every required parameter missing, and an
	// error earlier on the line wins over it.
	if got := help(t, []string{"-h"}, object.Args{files}, resize...); got != want {
		t.Error("-h differs from --help")
	}
	_, err := parse(t, []string{"--nope", "--help"}, object.Args{files}, resize...)
	usageError(t, err)
	if _, err := parse(t, []string{"--", "--help"}, object.Args{files}, resize...); !errors.Is(err, UsageError) {
		t.Errorf("--help after -- = %v", err)
	}

	tool := []object.KwArg{
		kw("build", construct(t, "Command", nil, kw("release", object.Bool(false)), kw("help", str("Compile the project")))),
		kw("test", construct(t, "Command", object.Args{construct(t, "Arg", object.Args{str("name")}, kw("default", object.Nil))}, kw("help", str("Run the tests")))),
		kw("verbose", construct(t, "Flag", nil, kw("short", str("v")))),
		kw("help", str("Project tool.")),
	}
	wantTool := `usage: tool [options] <command> ...

Project tool.

commands:
  build          Compile the project
  test           Run the tests

options:
  -v, --verbose
  -h, --help     show this help and exit`
	if got := help(t, []string{"--help"}, nil, tool...); got != wantTool {
		t.Errorf("help:\n%s\nwant:\n%s", got, wantTool)
	}
	wantBuild := `usage: tool build [options]

Compile the project

options:
      --release
  -v, --verbose
  -h, --help     show this help and exit`
	if got := help(t, []string{"build", "-h"}, nil, tool...); got != wantBuild {
		t.Errorf("help:\n%s\nwant:\n%s", got, wantBuild)
	}
	_, err = parse(t, []string{"test", "a", "b"}, nil, tool...)
	if msg, usage := usageError(t, err); msg != `unexpected argument "b"` || usage != "usage: tool test [options] [<name>]" {
		t.Errorf("%s | %s", msg, usage)
	}
}

func TestDeclarationErrors(t *testing.T) {
	cases := []struct {
		pos  object.Args
		kws  []object.KwArg
		kind *object.Error
		want string
	}{
		{nil, []object.KwArg{kw("x", &object.Dict{})}, object.TypeError, "parse() option 'x' must be false, nil, String, Int, Float, List, Flag, Option or Command, got Dict"},
		{nil, []object.KwArg{kw("x", object.Bool(true))}, object.ValueError, "parse() option 'x' cannot default to true; declare the opposite flag (no_x=false)"},
		{object.Args{object.Integer(1)}, nil, object.TypeError, "parse() argument 1 must be String or Arg, got Int"},
		{nil, []object.KwArg{kw("Dry-Run", object.Bool(false))}, object.ValueError, "parse() invalid name 'Dry-Run' (use lower case letters, digits and '_')"},
		{nil, []object.KwArg{kw("a__b", object.Bool(false))}, object.ValueError, "parse() invalid name 'a__b' (use lower case letters, digits and '_')"},
		{object.Args{str("output")}, []object.KwArg{kw("output", object.Nil)}, object.ValueError, "parse() duplicate name 'output'"},
		{object.Args{str("a")}, []object.KwArg{kw("b", construct(t, "Command", nil))}, object.ValueError, "parse() cannot combine arguments with commands"},
	}
	for _, c := range cases {
		// A wrong declaration is reported whatever the command line, --help
		// included.
		_, err := parse(t, []string{"--help"}, c.pos, c.kws...)
		if !errors.Is(err, c.kind) || err.Error() != c.want {
			t.Errorf("error = %v, want %s", err, c.want)
		}
	}
	m := module(t, "tool")
	if _, err := m.Members["Option"].(*object.Function).Call(object.CallArgs{Keyword: object.Kwargs{kw("short", str("ab"))}}); err == nil ||
		err.Error() != `Option() argument 'short' must be one letter, got "ab"` {
		t.Errorf("short: %v", err)
	}
}

func TestArgs(t *testing.T) {
	args := mustParse(t, []string{"a", "b"}, object.Args{str("source"), str("target")}, kw("verbose", object.Bool(false)))
	if got := args.String(); got != `Args(source="a", target="b", verbose=false)` {
		t.Fatal(got)
	}
	if got := strings.Join(args.Attributes(), " "); got != "source target verbose" {
		t.Fatal(got)
	}
	if _, err := args.GetAttr("verbsoe"); err == nil || err.Error() != "Args has no attribute 'verbsoe'" {
		t.Fatal(err)
	}
	if _, err := args.SetAttr("verbose", object.Bool(true)); err == nil || err.Error() != "Args is read-only" {
		t.Fatal(err)
	}
	// A many default is a new List on every parse.
	def := strList(".")
	paths := construct(t, "Arg", object.Args{str("paths")}, kw("default", def), kw("many", object.Bool(true)))
	got, _ := mustParse(t, nil, object.Args{paths}).GetAttr("paths")
	if got == object.Object(def) || !modtest.Equal(got, def) {
		t.Fatalf("paths = %v", got)
	}
}

// processMode replaces the streams for one test.
func processMode(t *testing.T) (out, errOut *bytes.Buffer) {
	t.Helper()
	out, errOut = &bytes.Buffer{}, &bytes.Buffer{}
	oldOut, oldErr := stdout, stderr
	stdout = func() io.Writer { return out }
	stderr = func() io.Writer { return errOut }
	t.Cleanup(func() { stdout, stderr = oldOut, oldErr })
	return out, errOut
}

// processParse calls parse of the module built over argv. In process mode
// help and usage errors end the program through an exit request, so the
// status is read from the error: -1 means parse returned without one.
func processParse(t *testing.T, argv []string, call object.CallArgs) (object.Object, int) {
	t.Helper()
	v, err := module(t, argv...).Members["parse"].(*object.Function).Call(call)
	if err == nil {
		return v, -1
	}
	code, ok := object.ExitCode(err)
	if !ok {
		t.Fatalf("parse() error = %v", err)
	}
	if v != nil {
		t.Fatalf("parse() returned %v with an exit request", v)
	}
	return v, code
}

func TestProcessMode(t *testing.T) {
	call := object.CallArgs{Positional: object.Args{str("source"), str("target")}, Keyword: object.Kwargs{kw("help", str("Copy."))}}

	out, errOut := processMode(t)
	v, status := processParse(t, []string{"/bin/copy", "a", "b"}, call)
	if status != -1 || v.(*Args).String() != `Args(source="a", target="b")` || out.Len()+errOut.Len() != 0 {
		t.Fatalf("good line: %v %d %q %q", v, status, out, errOut)
	}

	out, errOut = processMode(t)
	_, status = processParse(t, []string{"scripts/copy.goblin", "a"}, call)
	wantErr := "error: missing argument <target>\nusage: copy.goblin [options] <source> <target>\nRun 'copy.goblin --help' for details.\n"
	if status != 2 || out.Len() != 0 || errOut.String() != wantErr {
		t.Fatalf("usage error: %d %q %q", status, out, errOut)
	}

	out, errOut = processMode(t)
	if _, status = processParse(t, nil, object.CallArgs{Keyword: object.Kwargs{kw("argv", object.Nil)}}); status != -1 {
		t.Fatalf("empty line exited %d", status)
	}
	_, status = processParse(t, []string{"", "-h"}, object.CallArgs{})
	wantHelp := "usage: program [options]\n\noptions:\n  -h, --help  show this help and exit\n"
	if status != 0 || errOut.Len() != 0 || out.String() != wantHelp {
		t.Fatalf("help: %d %q %q", status, out, errOut)
	}
}

func TestExecuteWithFrozenArgs(t *testing.T) {
	m := module(t, "/opt/tools/deploy", "--dry-run")
	v, err := m.Members["parse"].(*object.Function).Call(object.CallArgs{Keyword: object.Kwargs{kw("dry_run", object.Bool(false))}})
	if err != nil || v.(*Args).String() != "Args(dry_run=true)" {
		t.Fatalf("%v %v", v, err)
	}
	// argv= replaces the frozen command line entirely; the name stays.
	_, err = m.Members["parse"].(*object.Function).Call(object.CallArgs{Keyword: object.Kwargs{kw("argv", strList("x"))}})
	if msg, usage := usageError(t, err); msg != `unexpected argument "x"` || usage != "usage: deploy [options]" {
		t.Fatalf("%s | %s", msg, usage)
	}
}

func TestValidName(t *testing.T) {
	for name, want := range map[string]bool{
		"a": true, "dry_run": true, "x2": true, "a_1_b": true, "utf8_name": true,
		"": false, "2x": false, "_a": false, "a_": false, "a__b": false, "Dry": false, "dry-run": false, "é": false,
	} {
		if validName(name) != want {
			t.Errorf("validName(%q) = %v", name, !want)
		}
	}
}
