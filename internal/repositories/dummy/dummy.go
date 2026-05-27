package dummyRepository

import (
	"github.com/Open-Zord/zord/internal/application/domain/dummy"
	"github.com/Open-Zord/zord/internal/repositories/base_repository"

	"github.com/jmoiron/sqlx"
)

// RegistryKey is the key under which the dummy repository is registered in the registry.
const RegistryKey = "dummyRepository"

type DummyRepository struct {
	*base_repository.BaseRepo[dummy.Dummy]
}

func NewDummyRepository(mysql *sqlx.DB) *DummyRepository {
	return &DummyRepository{
		BaseRepo: base_repository.NewBaseRepository[dummy.Dummy](mysql),
	}
}
