package http

import (
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/domain/dummy"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/providers/baserepo"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/providers/baseservice"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/services/dummy/create"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/services/dummy/delete"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/services/dummy/edit"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/services/dummy/get"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/services/dummy/list"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/repositories/baserepository"
	dummyrepo "github.com/Open-Zord/zord/internal/skeleton/runtime/internal/repositories/dummy"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/idcreator"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/logger"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/registry"
)

// registerServices builds and registers the application services (use cases)
// into the registry. Services resolve their primitives and repositories from
// the registry and are constructed eagerly, so a missing dependency fails fast
// at boot. The scaffold tool appends new service registrations here.
func registerServices(reg *registry.Registry) {
	log := registry.Resolve[baseservice.Logger](reg, logger.RegistryKey)
	idC := registry.Resolve[baseservice.IDCreator](reg, idcreator.RegistryKey)
	dummyRepo := registry.Resolve[dummy.Repository](reg, dummyrepo.RegistryKey)
	uow := registry.Resolve[baserepo.UnitOfWork](reg, baserepository.UoWRegistryKey)
	reg.Provide(create.RegistryKey, create.NewService(log, idC, dummyRepo, uow))
	reg.Provide(get.RegistryKey, get.NewService(log, idC))
	reg.Provide(list.RegistryKey, list.NewService(log, idC))
	reg.Provide(edit.RegistryKey, edit.NewService(log, idC))
	reg.Provide(delete.RegistryKey, delete.NewService(log, idC))
}
