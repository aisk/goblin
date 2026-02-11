package object

// ModuleExecutor is the signature of a module's Execute function.
type ModuleExecutor func() (Object, error)

// Registry is a module registry that caches loaded modules by path.
type Registry struct {
	modules map[string]Object
}

// NewRegistry creates a new module registry.
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]Object),
	}
}

// Load loads a module: returns cached result if available, otherwise executes
// the executor and caches the result.
func (r *Registry) Load(path string, executor ModuleExecutor) (Object, error) {
	if mod, ok := r.modules[path]; ok {
		return mod, nil
	}
	mod, err := executor()
	if err != nil {
		return nil, err
	}
	r.modules[path] = mod
	return mod, nil
}

// Get returns a cached module. Returns nil, false if not found.
func (r *Registry) Get(path string) (Object, bool) {
	mod, ok := r.modules[path]
	return mod, ok
}
