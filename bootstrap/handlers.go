package bootstrap

import (
	"go-skeleton/pkg/registry"
)

// registerHandlers builds and registers the HTTP handlers into the registry.
// Each handler resolves its service(s) from the registry in its constructor, so
// a missing dependency fails fast at boot. The scaffold tool appends new
// handler registrations here.
func registerHandlers(reg *registry.Registry) {
}
