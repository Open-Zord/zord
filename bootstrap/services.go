package bootstrap

import (
	"go-skeleton/pkg/registry"
)

// registerServices builds and registers the application services (use cases)
// into the registry. Services resolve their primitives and repositories from
// the registry and are constructed eagerly, so a missing dependency fails fast
// at boot. The scaffold tool appends new service registrations here.
func registerServices(reg *registry.Registry) {
}
