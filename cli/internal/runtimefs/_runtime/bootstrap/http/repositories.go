package http

import (
	"github.com/Open-Zord/zord/internal/repositories/baserepository"
	dummyrepo "github.com/Open-Zord/zord/internal/repositories/dummy"
	"github.com/Open-Zord/zord/pkg/database"
	"github.com/Open-Zord/zord/pkg/registry"

	"github.com/jmoiron/sqlx"
)

// registerRepositories registers the persistence repositories and the shared
// UnitOfWork. Depends only on db (*sqlx.DB) already registered by pkg.go. The
// scaffold tool appends new repository registrations here.
func registerRepositories(reg *registry.Registry) {
	db := registry.Resolve[*sqlx.DB](reg, database.RegistryKey)

	reg.Provide(baserepository.UoWRegistryKey, baserepository.NewUoW(db))
	reg.Provide(dummyrepo.RegistryKey, dummyrepo.NewDummyRepository(db))
}
