// Package bootstrap assembles the full application dependency graph in
// pkg/registry and returns the ready-to-use registry for the HTTP server. It is
// the single authorized wiring point of the project — cmd, internal and pkg do
// not construct dependencies outside of here.
package bootstrap

import (
	"go-skeleton/pkg/registry"
)

// Setup loads configs, registers primitives, repositories, services and
// handlers in topological order and returns (registry, apiPrefix). The
// apiPrefix is propagated directly because the entrypoint needs it to build the
// server.
func Setup() (reg *registry.Registry, apiPrefix string) {
	conf, apiPrefix := loadConfigs()

	reg = registry.NewRegistry()
	registerPkg(reg, conf)
	registerRepositories(reg)
	registerServices(reg)
	registerHandlers(reg)

	return reg, apiPrefix
}
