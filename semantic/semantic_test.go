package semantic

import (
	"strings"
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/lexer"
	"github.com/aisk/goblin/parser"
)

func TestCheckModule(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantErr     bool
		errContains string
	}{
		{
			name:        "undefined identifier",
			source:      "print(x)\n",
			wantErr:     true,
			errContains: "undefined identifier: x",
		},
		{
			name:        "assignment to undefined identifier",
			source:      "x = 1\n",
			wantErr:     true,
			errContains: "assignment to undefined identifier: x",
		},
		{
			name:        "duplicate declaration",
			source:      "var a = 1\nvar a = 2\n",
			wantErr:     true,
			errContains: "duplicate declaration in same scope: a",
		},
		{
			name:    "shadowing in child scope is allowed",
			source:  "var a = 1\nif true { var a = 2 print(a) }\nprint(a)\n",
			wantErr: false,
		},
		{
			name:        "break outside loop",
			source:      "break\n",
			wantErr:     true,
			errContains: "break used outside loop",
		},
		{
			name:    "break inside loop",
			source:  "while true { break }\n",
			wantErr: false,
		},
		{
			name:    "loop body may shadow iteration variable",
			source:  "for x in [1] { var x = 2 print(x) }\n",
			wantErr: false,
		},
		{
			name:        "loop variable does not escape",
			source:      "for x in [1] { print(x) }\nprint(x)\n",
			wantErr:     true,
			errContains: "undefined identifier: x",
		},
		{
			name:        "return outside function",
			source:      "return 1\n",
			wantErr:     true,
			errContains: "return used outside function",
		},
		{
			name:    "return inside function",
			source:  "func f() { return 1 }\nprint(f())\n",
			wantErr: false,
		},
		{
			name:        "duplicate function parameter",
			source:      "func f(a, a) { return a }\n",
			wantErr:     true,
			errContains: "duplicate parameter name: a",
		},
		{
			name: "varargs function and starred call",
			source: "func f(a, *rest) {\n" +
				"  print(a)\n" +
				"  print(rest.size())\n" +
				"}\n" +
				"var xs = [2, 3]\n" +
				"f(1, *xs)\n",
			wantErr: false,
		},
		{
			name:        "undefined identifier in starred argument",
			source:      "func f(*args) { return nil }\nf(*missing)\n",
			wantErr:     true,
			errContains: "undefined identifier: missing",
		},
		{
			name: "keyword arguments call",
			source: "func f(a, b) {\n" +
				"  return a\n" +
				"}\n" +
				"f(a=1, b=2)\n",
			wantErr: false,
		},
		{
			name:        "positional after keyword argument",
			source:      "func f(a, b) { return a }\nf(a=1, 2)\n",
			wantErr:     true,
			errContains: "positional argument cannot appear after keyword arguments",
		},
		{
			name:        "duplicate keyword argument",
			source:      "func f(a) { return a }\nf(a=1, a=2)\n",
			wantErr:     true,
			errContains: "duplicate keyword argument: a",
		},
		{
			name: "starred argument after keyword argument",
			source: "func f(a, b) { return a }\n" +
				"var xs = [2]\n" +
				"f(a=1, *xs)\n",
			wantErr:     true,
			errContains: "positional argument cannot appear after keyword arguments",
		},
		{
			name: "args and kwargs parameters",
			source: "func f(a, *args, **kwargs) {\n" +
				"  return a\n" +
				"}\n" +
				"f(1, b=2)\n",
			wantErr: false,
		},
		{
			name:        "required parameter after args",
			source:      "func f(*args, a) { return a }\n",
			wantErr:     true,
			errContains: "args parameter must be the last parameter or followed by kwargs",
		},
		{
			name:        "kwargs must be last",
			source:      "func f(**kwargs, a) { return a }\n",
			wantErr:     true,
			errContains: "kwargs parameter must be the last parameter",
		},
		{
			name:        "import name conflict",
			source:      "import \"os\"\nvar os = 1\n",
			wantErr:     true,
			errContains: "duplicate declaration in same scope: os",
		},
		{
			name:        "export undefined",
			source:      "export missing\n",
			wantErr:     true,
			errContains: "export of undefined identifier: missing",
		},
		{
			name: "type with self method",
			source: "type User(name, age=18) {\n" +
				"  func hello(self) {\n" +
				"    print(self.name)\n" +
				"  }\n" +
				"}\n" +
				"var user = User(\"alice\")\n" +
				"user.hello()\n",
			wantErr: false,
		},
		{
			name: "type duplicate field",
			source: "type User(name, name) {\n" +
				"  func hello(self) { return nil }\n" +
				"}\n",
			wantErr:     true,
			errContains: "duplicate type field name: name",
		},
		{
			name: "bare field reference in method is undefined",
			source: "type User(name) {\n" +
				"  func hello(self) { return name }\n" +
				"}\n",
			wantErr:     true,
			errContains: "undefined identifier: name",
		},
		{
			name: "method parameter may share a field's name",
			source: "type User(name) {\n" +
				"  func rename(self, name) { self.name = name }\n" +
				"}\n",
			wantErr: false,
		},
		{
			name: "type method requires self",
			source: "type User(name) {\n" +
				"  func hello(name) { print(name) }\n" +
				"}\n",
			wantErr:     true,
			errContains: "type method must declare 'self' as the first parameter",
		},
		{
			name: "required type field after default",
			source: "type User(age=18, name) {\n" +
				"  func hello(self) { print(self.name) }\n" +
				"}\n",
			wantErr:     true,
			errContains: "required type field cannot appear after default field: name",
		},
		{
			name: "protocol method wrong arity",
			source: "type V(x) {\n" +
				"  func __add(self) { return self }\n" +
				"}\n",
			wantErr:     true,
			errContains: "protocol method '__add' must declare exactly 2 parameters including self, got 1",
		},
		{
			name: "protocol method with varargs rejected",
			source: "type V(x) {\n" +
				"  func __cmp(self, *rest) { return 0 }\n" +
				"}\n",
			wantErr:     true,
			errContains: "protocol method '__cmp' cannot use variadic or keyword parameters",
		},
		{
			name: "protocol method correct arity accepted",
			source: "type V(x) {\n" +
				"  func __add(self, other) { return self }\n" +
				"  func __str(self) { return \"v\" }\n" +
				"  func __setitem(self, i, val) { return nil }\n" +
				"}\n" +
				"print(V(1))\n",
			wantErr: false,
		},
		{
			name: "non-protocol method named add is unrestricted",
			source: "type V(x) {\n" +
				"  func add(self, a, b, c) { return self }\n" +
				"}\n" +
				"print(V(1))\n",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod := parseModule(t, tt.source)
			err := CheckModule(mod)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if tt.wantErr && tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
			}
		})
	}
}

func parseModule(t *testing.T, source string) *ast.Module {
	t.Helper()

	l := lexer.NewLexer([]byte(source))
	p := parser.NewParser()
	st, err := p.Parse(l)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	mod, ok := st.(*ast.Module)
	if !ok {
		t.Fatalf("failed to convert AST to Module")
	}
	return mod
}
