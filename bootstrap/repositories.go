package bootstrap

import (
	dummyRepository "go-skeleton/internal/repositories/dummy"
	"go-skeleton/pkg/database"
	"go-skeleton/pkg/registry"

	"github.com/jmoiron/sqlx"
)

// registerRepositories registers the persistence repositories. Depends only on
// db (*sqlx.DB) already registered by pkg.go. The scaffold tool appends new
// repository registrations here.
func registerRepositories(reg *registry.Registry) {
	db := registry.Resolve[*sqlx.DB](reg, database.RegistryKey)

	reg.Provide(dummyRepository.RegistryKey, dummyRepository.NewDummyRepository(db))
}
