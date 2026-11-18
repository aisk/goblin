package semantic

import "testing"

func TestStaticCallAndAttributeChecks(t *testing.T) {
	const point = "type P(x, y = 0) {\n  func move(self, dx, dy = 0) {\n    self.x = self.x + dx\n  }\n}\n"
	tests := []struct {
		name   string
		source string
		err    string // empty: the module must pass
	}{
		// Function arity.
		{"exact arity", "func f(a, b) {}\nf(1, 2)\n", ""},
		{"too many arguments", "func f(a) {}\nf(1, 2)\n", "2:1: semantic error: f() takes 1 positional arguments, got 2"},
		{"too few arguments", "func f(a, b) {}\nf(1)\n", "2:1: semantic error: f() missing required argument: 'b'"},
		{"default may be omitted", "func f(a, b = 1) {}\nf(1)\n", ""},
		{"keyword fills a required parameter", "func f(a, b) {}\nf(1, b=2)\n", ""},
		{"unknown keyword", "func f(a) {}\nf(1, c=2)\n", "2:6: semantic error: f() got an unexpected keyword argument 'c'"},
		{"keyword repeats a positional", "func f(a, b = 1) {}\nf(1, a=2)\n", "2:6: semantic error: f() got multiple values for argument 'a'"},
		{"varargs take the surplus", "func f(a, *rest) {}\nf(1, 2, 3)\n", ""},
		{"varargs still need the fixed parameters", "func f(a, *rest) {}\nf()\n", "2:1: semantic error: f() missing required argument: 'a'"},
		{"kwargs take unknown keywords", "func f(a, **kw) {}\nf(1, c=2)\n", ""},
		{"starred argument may supply the rest", "func f(a, b) {}\nvar xs = [1, 2]\nf(*xs)\n", ""},
		{"keyword unpacking may supply the rest", "func f(a, b) {}\nvar kw = {}\nf(1, **kw)\n", ""},
		{"too many even before a starred argument", "func f(a) {}\nvar xs = []\nf(1, 2, *xs)\n", "3:1: semantic error: f() takes 1 positional arguments, got 2"},
		{"call before the definition", "func g() {\n  return f(1, 2)\n}\nfunc f(a) {}\n", "2:10: semantic error: f() takes 1 positional arguments, got 2"},
		{"recursive call", "func f(a) {\n  return f()\n}\n", "2:10: semantic error: f() missing required argument: 'a'"},
		{"nested function", "func g() {\n  func h(a) {}\n  h()\n}\n", "3:3: semantic error: h() missing required argument: 'a'"},
		{"function literal in a variable", "var f = func(a) {}\nf()\n", "2:1: semantic error: <lambda>() missing required argument: 'a'"},
		{"deferred call", "func f(a) {}\nfunc g() {\n  defer f()\n}\n", "3:9: semantic error: f() missing required argument: 'a'"},

		// Anything that may hold another value stays dynamic.
		{"reassigned function", "func f(a) {}\nf = func() {}\nf()\n", ""},
		{"function reassigned later, elsewhere", "func f(a) {}\nfunc g() {\n  f()\n}\nfunc h() {\n  f = func() {}\n}\n", ""},
		{"reassigned function variable", "var f = func(a) {}\nf = func() {}\nf()\n", ""},
		{"destructuring assignment rebinds", "func f(a) {}\nvar g = 0\nf, g = [func() {}, 1]\nf()\n", ""},
		{"parameter shadows the function", "func f(a) {}\nfunc g(f) {\n  f()\n}\n", ""},
		{"local shadows the function", "func f(a) {}\nfunc g() {\n  var f = [func() {}][0]\n  f()\n}\n", ""},
		{"function passed as a value", "func f(a) {}\nfunc call(g) {\n  return g()\n}\ncall(f)\n", ""},

		// Constructors.
		{"construction", point + "P(1)\nP(1, 2)\nP(y=2, x=1)\n", ""},
		{"construction with too many fields", point + "P(1, 2, 3)\n", "6:1: semantic error: P() takes 2 positional arguments, got 3"},
		{"construction missing a field", point + "P()\n", "6:1: semantic error: P() missing required argument: 'x'"},
		{"construction with an unknown field", point + "P(1, z=2)\n", "6:6: semantic error: P() got an unexpected keyword argument 'z'"},
		{"local shadows the type", point + "func g() {\n  var P = func() {}\n  P()\n}\n", ""},

		// Attributes of known instances.
		{"self field and method", point + "", ""},
		{"unknown attribute of self", "type P(x) {\n  func f(self) {\n    return self.z\n  }\n}\n", "3:17: semantic error: P has no attribute 'z'"},
		{"store to an unknown attribute of self", "type P(x) {\n  func f(self) {\n    self.z = 1\n  }\n}\n", "3:10: semantic error: P has no attribute 'z'"},
		{"store to a method", "type P(x) {\n  func f(self) {\n    self.f = 1\n  }\n}\n", "3:10: semantic error: cannot assign to method 'f' of P"},
		{"self in an impl method", "type P(x) {\n  impl Show {\n    func show(self) {\n      return self.z\n    }\n  }\n}\n", "4:19: semantic error: P has no attribute 'z'"},
		{"self captured by a closure", "type P(x) {\n  func f(self) {\n    return func() { return self.z }\n  }\n}\n", "3:33: semantic error: P has no attribute 'z'"},
		{"no built-in attributes", "type P(x) {\n  func f(self) {\n    return self.constructor\n  }\n}\n", "3:17: semantic error: P has no attribute 'constructor'"},
		{"reassigned self", "type P(x) {\n  func f(self) {\n    self = 1\n    return self.z\n  }\n}\n", ""},
		{"another parameter named like a field holder", "type P(x) {\n  func f(self, other) {\n    return other.z\n  }\n}\n", ""},
		{"attribute of a construction", point + "print(P(1).z)\n", "6:12: semantic error: P has no attribute 'z'"},
		{"attribute of a variable holding a construction", point + "var p = P(1)\nprint(p.x, p.y)\nprint(p.z)\n", "8:9: semantic error: P has no attribute 'z'"},
		{"store through such a variable", point + "var p = P(1)\np.z = 1\n", "7:3: semantic error: P has no attribute 'z'"},
		{"reassigned variable", point + "var p = P(1)\np = {\"z\": 1}\nprint(p.z)\n", ""},
		{"variable reassigned in a function", point + "var p = P(1)\nfunc g() {\n  p = 1\n}\nprint(p.z)\n", ""},
		{"variable copied from another", point + "var p = P(1)\nvar q = p\nprint(q.z)\n", ""},

		// Method arity.
		{"method call", point + "var p = P(1)\np.move(1)\np.move(1, 2)\np.move(dx=1)\n", ""},
		{"method with too many arguments", point + "var p = P(1)\np.move(1, 2, 3)\n", "7:3: semantic error: P.move() takes 2 positional arguments, got 3"},
		{"method missing an argument", point + "P(1).move()\n", "6:6: semantic error: P.move() missing required argument: 'dx'"},
		{"method called through self", "type P(x) {\n  func a(self, n) {}\n  func b(self) {\n    self.a()\n  }\n}\n", "4:10: semantic error: P.a() missing required argument: 'n'"},
		{"function held in a field", "type H(f) {\n  func run(self) {\n    return self.f(1, 2, 3)\n  }\n}\n", ""},
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
