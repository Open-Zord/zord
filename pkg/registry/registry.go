// Package registry implements a simple service locator used by the bootstrap
// package to register dependencies and by consumers (handlers, services, etc.)
// to resolve them at runtime.
package registry

import "fmt"

type Registry struct {
	deps map[string]Dependency
}

type Dependency interface{}

func NewRegistry() *Registry {
	return &Registry{
		deps: make(map[string]Dependency),
	}
}

// Provide registra dep sob name. Panica se name já estiver registrado:
// overwrite silencioso era um bug de wiring difícil de rastrear (o último
// registro vencia sem aviso). Fail-fast no boot.
func (r *Registry) Provide(name string, dep Dependency) {
	if _, dup := r.deps[name]; dup {
		panic(fmt.Sprintf("registry: %q já registrado (overwrite não permitido)", name))
	}
	r.deps[name] = dep
}

// Inject resolves a dependency by name and returns it without casting. Panics
// if the key is not registered. Prefer Resolve[T] when you know the expected
// type — this version is a fallback for callers that want the raw Dependency.
func (r *Registry) Inject(name string) Dependency {
	dep, ok := r.deps[name]
	if !ok {
		panic(fmt.Sprintf("registry: %q not registered", name))
	}
	return dep
}

// Resolve resolves a dependency by name and returns it typed as T. Panics if
// the key is not registered (via Inject) OR if the registered type is not
// assignable to T. The panic message includes the key, the registered type and
// the expected type to aid diagnosis.
//
// It is a free function (not a method) because Go methods cannot have type
// parameters — the generic must live in the package namespace.
func Resolve[T any](r *Registry, name string) T {
	dep := r.Inject(name)
	t, ok := dep.(T)
	if !ok {
		panic(fmt.Sprintf(
			"registry: %q has type %T, expected %T",
			name, dep, *new(T),
		))
	}
	return t
}

// Validate confere invariantes do grafo montado, como gate de boot: hoje, que
// nenhuma dependência foi registrada como nil. A completude do grafo (todas as
// deps necessárias presentes) já é garantida pela construção eager no Setup, que
// resolve tudo e panica se faltar — Validate cobre o complemento.
func (r *Registry) Validate() error {
	for name, dep := range r.deps {
		if dep == nil {
			return fmt.Errorf("registry: %q registrado como nil", name)
		}
	}
	return nil
}
