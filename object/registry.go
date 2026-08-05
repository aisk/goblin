package object

// ModuleExecutor is the signature of a module's Execute function.
type ModuleExecutor func() (Object, error)

// Registry is a module registry that caches loaded modules by path.
type Registry struct {
	modules map[string]Object
	loading map[string]struct{}
}

// NewRegistry creates a new module registry.
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]Object),
		loading: make(map[string]struct{}),
	}
}

// Load loads a module: returns cached result if available, otherwise executes
// the executor and caches the result. Re-entering Load for a path whose
// executor is still running is a circular import.
func (r *Registry) Load(path string, executor ModuleExecutor) (Object, error) {
	if mod, ok := r.modules[path]; ok {
		return mod, nil
	}
	if _, ok := r.loading[path]; ok {
		return nil, NewImportError("circular import detected: %s", path)
	}
	r.loading[path] = struct{}{}
	defer delete(r.loading, path)
	mod, err := executor()
	if err != nil {
		return nil, err
	}
	if m, ok := mod.(*Module); ok && m.Name == "" {
		m.Name = path
	}
	r.modules[path] = mod
	return mod, nil
}

// Get returns a cached module. Returns nil, false if not found.
func (r *Registry) Get(path string) (Object, bool) {
	mod, ok := r.modules[path]
	return mod, ok
}
