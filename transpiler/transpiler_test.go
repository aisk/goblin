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

func TestTranspileStaticMemberCallUsesDirectReceiverMethod(t *testing.T) {
	cases := []struct {
		name       string
		source     string
		wantMethod string
	}{
		{
			name:       "list literal",
			source:     "print([1, 2].push(3))\n",
			wantMethod: ".Push(",
		},
		{
			name:       "dict literal",
			source:     "print({\"a\": 1}.keys())\n",
			wantMethod: ".Keys(",
		},
		{
			name:       "string literal",
			source:     "print(\" x \".trim_space())\n",
			wantMethod: ".TrimSpace(",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := transpileSource(t, tc.source)
			if !strings.Contains(code, tc.wantMethod) {
				t.Fatalf("expected transpiled code to contain %q\n%s", tc.wantMethod, code)
			}
			if strings.Contains(code, ".GetAttr(") {
				t.Fatalf("expected static member call to skip GetAttr\n%s", code)
			}
		})
	}
}

func TestTranspileDynamicMemberCallFallsBackToGetAttr(t *testing.T) {
	code := transpileSource(t, "var xs = [1, 2]\nprint(xs.push(3))\n")

	if !strings.Contains(code, ".GetAttr(") {
		t.Fatalf("expected transpiled code to use GetAttr fallback\n%s", code)
	}
	if !strings.Contains(code, "object.Call") {
		t.Fatalf("expected transpiled code to call object.Call fallback\n%s", code)
	}
}

func TestTranspileTypeDefineGeneratesStructAndMethods(t *testing.T) {
	code := transpileSource(t, "type User(name, age=18) {\n  func hello(self) { print(self.name) }\n}\nvar user = User(\"alice\")\nuser.hello()\n")

	for _, want := range []string{
		"type _goblin_type_User_0 struct",
		"func (u *_goblin_type_User_0) Hello(",
		"func (u *_goblin_type_User_0) GetAttr(",
		`case "name":`,
		`case "hello":`,
		`var User object.Object = &object.Function{`,
		`_instance._method_hello = _instance.Hello`,
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("expected transpiled code to contain %q\n%s", want, code)
		}
	}
}
