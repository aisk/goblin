package transpiler

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
	"github.com/aisk/goblin/semantic"
)

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
			source:   "print(\" x \".trim_space())\n",
			wantAttr: `.GetAttr("trim_space")`,
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

func TestTranspileTypeDefineGeneratesStructAndMethods(t *testing.T) {
	code := transpileSource(t, "type User(name, age=18) {\n  func hello(self) { print(self.name) }\n}\nvar user = User(\"alice\")\nuser.hello()\n")

	for _, want := range []string{
		"type User struct",
		"func (u *User) Hello(",
		"func (u *User) GetAttr(",
		`fmt.Sprintf("<User@%p>", u)`,
		`case "name":`,
		`case "hello":`,
		`case "constructor":`,
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
	if strings.Contains(code, "Repr()") {
		t.Fatalf("expected transpiled code to no longer generate Repr methods\n%s", code)
	}
}
