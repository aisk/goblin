package transpiler

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
	"github.com/aisk/goblin/source"
)

func TestGoblinRuntimeVersionFromBuildInfo(t *testing.T) {
	tests := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{
			name: "installed module version",
			info: &debug.BuildInfo{Main: debug.Module{
				Path:    pathBase,
				Version: "v1.2.3",
			}},
			want: "v1.2.3",
		},
		{
			name: "development build",
			info: &debug.BuildInfo{Main: debug.Module{
				Path:    pathBase,
				Version: "(devel)",
			}},
			want: defaultGoblinRuntimeVersion,
		},
		{
			name: "different main module",
			info: &debug.BuildInfo{Main: debug.Module{
				Path:    "example.com/wrapper",
				Version: "v1.2.3",
			}},
			want: defaultGoblinRuntimeVersion,
		},
		{
			name: "missing build info",
			want: defaultGoblinRuntimeVersion,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := goblinRuntimeVersionFromBuildInfo(tc.info); got != tc.want {
				t.Fatalf("goblinRuntimeVersionFromBuildInfo() = %q, want %q", got, tc.want)
			}
		})
	}
}

func transpileSource(t *testing.T, source string) string {
	t.Helper()

	l := lexer.NewLexer([]byte(source))

	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	mod, ok := st.(*ast.Module)
	if !ok {
		t.Fatalf("unexpected AST type %T", st)
	}

	if err := semantic.CheckModule(mod); err != nil {
		t.Fatalf("semantic error: %v", err)
	}

	var buf bytes.Buffer
	if err := Transpile(mod, &buf); err != nil {
		t.Fatalf("transpile error: %v", err)
	}

	return buf.String()
}

func TestTranspileMemberCallUsesGetAttr(t *testing.T) {
	cases := []struct {
		name     string
		source   string
		wantAttr string
	}{
		{
			name:     "list literal",
			source:   "print([1, 2].push(3))\n",
			wantAttr: `.GetAttr("push")`,
		},
		{
			name:     "dict literal",
			source:   "print({\"a\": 1}.keys())\n",
			wantAttr: `.GetAttr("keys")`,
		},
		{
			name:     "string literal",
			source:   "print(\" x \".trim())\n",
			wantAttr: `.GetAttr("trim")`,
		},
		{
			name:     "variable receiver",
			source:   "var xs = [1, 2]\nprint(xs.push(3))\n",
			wantAttr: `.GetAttr("push")`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := transpileSource(t, tc.source)
			if !strings.Contains(code, tc.wantAttr) {
				t.Fatalf("expected transpiled code to contain %q\n%s", tc.wantAttr, code)
			}
			if !strings.Contains(code, "object.Call") {
				t.Fatalf("expected transpiled code to call object.Call fallback\n%s", code)
			}
		})
	}
}

func TestTranspileKnownHTTPModuleImport(t *testing.T) {
	code := transpileSource(t, "import \"http\"\n")

	for _, want := range []string{
		`_registry.Load("http", http.Execute)`,
		`http_module`,
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("expected transpiled code to contain %q\n%s", want, code)
		}
	}
}

func TestTranspileFunctionsAttachGoblinFrames(t *testing.T) {
	code := transpileSource(t, "func fail() { return 1 / 0 }\nfail()\n")
	for _, want := range []string{
		"object.WithFrame",
		`Function: "fail"`,
		`Function: "<module>"`,
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("expected transpiled code to contain %q\n%s", want, code)
		}
	}
	if strings.Contains(code, "errors.WithStack") {
		t.Fatalf("generated code still uses errors.WithStack\n%s", code)
	}
}

// Frames must carry the module name derived from the source file, matching
// the interpreter's tracebacks (`at fail [mymod] (mymod.goblin:1:6)`).
func TestTranspiledFramesCarryModuleName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mymod.goblin")
	prog := "type Box(v) {\n  func get(self) { return self.v }\n}\nfunc fail() { return 1 / 0 }\nfail()\n"
	if err := os.WriteFile(path, []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}

	l, err := source.NewLexerFile(path)
	if err != nil {
		t.Fatal(err)
	}
	st, err := parser.NewParser().Parse(l)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	mod := st.(*ast.Module)
	if err := semantic.CheckModule(mod); err != nil {
		t.Fatalf("semantic error: %v", err)
	}
	var buf bytes.Buffer
	if err := Transpile(mod, &buf); err != nil {
		t.Fatalf("transpile error: %v", err)
	}
	code := buf.String()

	for _, want := range []*regexp.Regexp{
		regexp.MustCompile(`Module:\s+"mymod"`),
		regexp.MustCompile(`Function:\s+"Box\.get"`),
	} {
		if !want.MatchString(code) {
			t.Fatalf("expected transpiled code to match %q\n%s", want, code)
		}
	}
	if regexp.MustCompile(`Module:\s+""`).MatchString(code) {
		t.Fatalf("generated frames still carry an empty module name\n%s", code)
	}
}

func TestTranspileTypeDefineGeneratesStructAndMethods(t *testing.T) {
	code := transpileSource(t, "type User(name, age=18) {\n  func hello(self) { print(self.name) }\n}\nvar user = User(\"alice\")\nuser.hello()\n")

	for _, want := range []string{
		"type User struct",
		"func (u *User) Hello(",
		"func (u *User) GetAttr(",
		"func (u *User) Attributes() []string",
		`fmt.Sprintf("<User@%p>", u)`,
		`case "name":`,
		`case "hello":`,
		`case "constructor":`,
		`case "attributes":`,
		`return []string{"name", "age", "hello", "constructor", "attributes"}`,
		`var UserConstructor object.Object`,
		`UserConstructor = &object.Function{`,
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("expected transpiled code to contain %q\n%s", want, code)
		}
	}
	if strings.Contains(code, "_method_") {
		t.Fatalf("expected transpiled code to no longer reference _method_ slots\n%s", code)
	}
}

func TestTranspileTypeAllowsAttributesOverride(t *testing.T) {
	code := transpileSource(t, `type User() { func attributes(self) { return ["custom"] } }`)
	if strings.Contains(code, `case "attributes": {
		return object.AttributesFunction`) {
		t.Fatalf("generated default attributes method despite user override\n%s", code)
	}
	if !strings.Contains(code, `return []string{"attributes", "constructor"}`) {
		t.Fatalf("generated Attributes metadata does not include override\n%s", code)
	}
}
