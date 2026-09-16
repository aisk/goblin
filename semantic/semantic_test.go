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
			name:        "unused expression value",
			source:      "var a = 10\n- 2\nprint(a)\n",
			wantErr:     true,
			errContains: "2:1: semantic error: expression value is not used",
		},
		{
			name:        "unused binary expression",
			source:      "var a = 1\nvar b = 2\na + b\n",
			wantErr:     true,
			errContains: "expression value is not used",
		},
		{
			name:    "call, index and member expressions may stand alone",
			source:  "var l = [1]\nvar s = \"a\"\nprint(l)\nl[0]\ns.len\n",
			wantErr: false,
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
			source:  "var a = 1\nif true {\n\tvar a = 2\n\tprint(a)\n}\nprint(a)\n",
			wantErr: false,
		},
		{
			name:        "break outside loop",
			source:      "break\n",
			wantErr:     true,
			errContains: "break used outside loop",
		},
		{
			name:        "go keyword is reserved",
			source:      "var chan = 1\n",
			wantErr:     true,
			errContains: "'chan' is a reserved name",
		},
		{
			name:        "transpiler package alias is reserved",
			source:      "var object = 1\n",
			wantErr:     true,
			errContains: "'object' is a reserved name",
		},
		{
			name:        "transpiler helper name is reserved as parameter",
			source:      "func f(builtin) { return builtin }\n",
			wantErr:     true,
			errContains: "'builtin' is a reserved name",
		},
		{
			name:        "top-level wrapper name is reserved for types",
			source:      "type Execute(x) {}\n",
			wantErr:     true,
			errContains: "'Execute' is a reserved name",
		},
		{
			name:        "transpiler scratch name pattern is reserved",
			source:      "var _err_0 = 1\n",
			wantErr:     true,
			errContains: "'_err_0' is a reserved name",
		},
		{
			name:        "scratch name pattern applies to parameters",
			source:      "func f(_exports_1) { return _exports_1 }\n",
			wantErr:     true,
			errContains: "'_exports_1' is a reserved name",
		},
		{
			name:    "plain underscore names are not reserved",
			source:  "var _tmp = 1\nvar registry = 2\nvar self = 3\nprint(_tmp + registry + self)\n",
			wantErr: false,
		},
		{
			name:        "import inside function",
			source:      "func f() { import \"math\" }\n",
			wantErr:     true,
			errContains: "import is only allowed at module scope",
		},
		{
			name:        "type inside function",
			source:      "func f() { type T(x) {} }\n",
			wantErr:     true,
			errContains: "type is only allowed at module scope",
		},
		{
			name:        "export inside function",
			source:      "var x = 1\nfunc f() { export x }\n",
			wantErr:     true,
			errContains: "export is only allowed at module scope",
		},
		{
			name:        "duplicate type method name",
			source:      "type T(x) {\n  func m(self) { return 1 }\n  func m(self) { return 2 }\n}\n",
			wantErr:     true,
			errContains: "duplicate type method name: m",
		},
		{
			name:        "type method name conflicts with field name",
			source:      "type T(x) {\n  func x(self) { return 1 }\n}\n",
			wantErr:     true,
			errContains: "type method name conflicts with field name: x",
		},
		{
			name:        "continue outside loop",
			source:      "continue\n",
			wantErr:     true,
			errContains: "continue used outside loop",
		},
		{
			name:        "required parameter after args capture",
			source:      "func f(*args, x) { return x }\n",
			wantErr:     true,
			errContains: "args parameter must be the last parameter or followed by kwargs",
		},
		{
			name:        "parameter after kwargs capture",
			source:      "func f(**kw, x) { return x }\n",
			wantErr:     true,
			errContains: "kwargs parameter must be the last parameter",
		},
		{
			name:        "starred argument must be last",
			source:      "func f(a, b) { return a }\nf(*[1], 2)\n",
			wantErr:     true,
			errContains: "starred argument must be the last argument",
		},
		{
			name:    "break inside loop",
			source:  "while true { break }\n",
			wantErr: false,
		},
		{
			name:    "loop body may shadow iteration variable",
			source:  "for x in [1] {\n\tvar x = 2\n\tprint(x)\n}\n",
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
				"  print(rest.size)\n" +
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
			name:    "default parameters",
			source:  "var base = 1\nfunc f(a, b=base+1, *args, **kwargs) { return b }\nf(1)\n",
			wantErr: false,
		},
		{
			name:        "required parameter after default parameter",
			source:      "func f(a=1, b) { return b }\n",
			wantErr:     true,
			errContains: "required parameter cannot appear after default parameter: b",
		},
		{
			name:        "default expression sees enclosing scope only",
			source:      "func f(a, b=a) { return b }\n",
			wantErr:     true,
			errContains: "undefined identifier: a",
		},
		{
			name:        "default expression with undefined identifier",
			source:      "func f(a=missing) { return a }\n",
			wantErr:     true,
			errContains: "undefined identifier: missing",
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
			name: "dunder names are ordinary methods",
			source: "type V(x) {\n" +
				"  func __add(self) { return self }\n" +
				"}\n",
			wantErr: false,
		},
		{
			name: "method with default parameter",
			source: "type V(x) {\n" +
				"  func scale(self, factor=2) { return self }\n" +
				"}\n" +
				"print(V(1))\n",
			wantErr: false,
		},
		{
			name: "self with default rejected",
			source: "type V(x) {\n" +
				"  func m(self=1) { return self }\n" +
				"}\n",
			wantErr:     true,
			errContains: "type method must declare 'self' as the first parameter",
		},
		{
			name: "method named like a trait method is unrestricted",
			source: "type V(x) {\n" +
				"  impl Num { func add(self, other) { return self } }\n" +
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

func TestCheckTraits(t *testing.T) {
	tests := []struct {
		name   string
		source string
		// err is the expected message with its position, empty when the
		// module must pass.
		err string
	}{
		{
			name: "structural and custom impls",
			source: "type P(x) {\n" +
				"  impl Eq {}\n" +
				"  impl Ord {}\n" +
				"  impl Hashable {}\n" +
				"  impl Show { func show(self) { return \"p\" } }\n" +
				"  impl Num { func add(self, o) { return self } }\n" +
				"}\n",
		},
		{
			name: "user trait with defaults and dependencies",
			source: "trait Named: Show {\n" +
				"  func name(self)\n" +
				"  func greet(self) { return \"hi \" + Named.name(self) }\n" +
				"}\n" +
				"type P(x) {\n" +
				"  impl Named { func name(self) { return \"p\" } }\n" +
				"  impl Show {}\n" +
				"}\n",
		},
		{
			name: "mixin trait accepts an empty impl",
			source: "trait Loud { func shout(self) { return \"!\" } }\n" +
				"type P() { impl Loud {} }\n",
		},
		{
			name:   "missing required method",
			source: "trait Shape { func area(self) }\ntype P() { impl Shape {} }\n",
			err:    "2:17: semantic error: impl Shape for P is missing method 'area'",
		},
		{
			name:   "custom trait has no structural default",
			source: "type P() { impl Truth {} }\n",
			err:    "1:17: semantic error: impl Truth for P is missing method 'truth'",
		},
		{
			name:   "trait not defined",
			source: "type P() { impl Shape {} }\n",
			err:    "1:17: semantic error: impl Shape: Shape is not defined",
		},
		{
			name:   "trait used before its declaration",
			source: "type P() { impl Shape {} }\ntrait Shape {}\n",
			err:    "1:17: semantic error: impl Shape: Shape is not defined",
		},
		{
			name:   "impl of something that is not a trait",
			source: "func Shape() {}\ntype P() { impl Shape {} }\n",
			err:    "2:17: semantic error: impl Shape: Shape is not a trait",
		},
		{
			name:   "qualified trait needs an import",
			source: "type P() { impl shapes.Shape {} }\n",
			err:    "1:17: semantic error: impl shapes.Shape: shapes.Shape is not defined",
		},
		{
			name:   "qualified trait through an import is checked at runtime",
			source: "import \"./shapes\"\ntype P() { impl shapes.Shape {} }\n",
		},
		{
			name:   "method outside the trait",
			source: "type P() { impl Eq { func equal(self, o) { return true } } }\n",
			err:    "1:27: semantic error: impl Eq for P: Eq has no method 'equal'",
		},
		{
			name:   "wrong arity",
			source: "type P() { impl Eq { func eq(self) { return true } } }\n",
			err:    "1:27: semantic error: impl Eq for P: method 'eq' must declare 2 parameters including self, got 1",
		},
		{
			name:   "impl method without self",
			source: "type P() { impl Eq { func eq(a, b) { return true } } }\n",
			err:    "1:27: semantic error: trait method must declare 'self' as the first parameter",
		},
		{
			name:   "impl method with a default parameter",
			source: "type P() { impl Eq { func eq(self, o=1) { return true } } }\n",
			err:    "1:36: semantic error: trait method 'eq' cannot declare default parameter values",
		},
		{
			name:   "impl method with varargs",
			source: "type P() { impl Index { func get(self, *i) { return 1 } } }\n",
			err:    "1:41: semantic error: trait method 'get' cannot use variadic or keyword parameters",
		},
		{
			name:   "duplicate impl",
			source: "type P() {\n  impl Show {}\n  impl Show {}\n}\n",
			err:    "3:8: semantic error: duplicate impl Show for P",
		},
		{
			name:   "empty Num impl",
			source: "type P() { impl Num {} }\n",
			err:    "1:17: semantic error: impl Num for P defines no methods",
		},
		{
			name:   "missing dependency",
			source: "type P() { impl Hashable { func hash(self) { return 1 } } }\n",
			err:    "1:17: semantic error: impl Hashable for P requires impl Eq",
		},
		{
			name:   "user dependency is not auto-filled",
			source: "trait A {}\ntrait B: A {}\ntype P() { impl B {} }\n",
			err:    "3:17: semantic error: impl B for P requires impl A",
		},
		{
			name:   "Ord fills in Eq",
			source: "type P(x) {\n  impl Ord {}\n  impl Hashable {}\n}\n",
		},
		{
			name: "structural Hashable over a custom Eq",
			source: "type P(x) {\n" +
				"  impl Eq { func eq(self, o) { return true } }\n" +
				"  impl Hashable {}\n" +
				"}\n",
			err: "3:8: semantic error: structural Hashable requires structural Eq on P",
		},
		{
			name: "structural Ord over a custom Eq",
			source: "type P(x) {\n" +
				"  impl Ord {}\n" +
				"  impl Eq { func eq(self, o) { return true } }\n" +
				"}\n",
			err: "2:8: semantic error: structural Ord requires structural Eq on P",
		},
		{
			name: "structural Hashable over an Eq from a custom Ord",
			source: "type P(x) {\n" +
				"  impl Ord { func compare(self, o) { return 0 } }\n" +
				"  impl Hashable {}\n" +
				"}\n",
			err: "3:8: semantic error: structural Hashable requires structural Eq on P",
		},
		{
			name:   "trait only at module scope",
			source: "func f() {\n  trait T {}\n}\n",
			err:    "2:9: semantic error: trait is only allowed at module scope",
		},
		{
			name:   "duplicate trait method",
			source: "trait T {\n  func a(self)\n  func a(self)\n}\n",
			err:    "3:8: semantic error: duplicate trait method name: a",
		},
		{
			name:   "trait method without self",
			source: "trait T { func a(x) }\n",
			err:    "1:16: semantic error: trait method must declare 'self' as the first parameter",
		},
		{
			name:   "unknown dependency",
			source: "trait T: Missing {}\n",
			err:    "1:10: semantic error: trait T: Missing is not defined",
		},
		{
			name:   "default body is checked",
			source: "trait T { func a(self) { return missing } }\n",
			err:    "1:33: semantic error: undefined identifier: missing",
		},
		{
			name:   "trait name cannot be reassigned",
			source: "trait T {}\nT = 5\n",
			err:    "2:1: semantic error: cannot assign to trait T",
		},
		{
			name:   "type name cannot be reassigned",
			source: "type P() {}\nfunc f() {\n  P = 1\n}\n",
			err:    "3:3: semantic error: cannot assign to type P",
		},
		{
			name:   "a local may shadow a type name",
			source: "type P() {}\nfunc f() {\n  var P = 1\n  P = 2\n}\n",
		},
		{
			name:   "duplicate trait name",
			source: "trait T {}\ntype T() {}\n",
			err:    "2:6: semantic error: duplicate declaration in same scope: T",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckModule(parseModule(t, tt.source))
			if tt.err == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.err {
				t.Fatalf("error = %v, want %q", err, tt.err)
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
