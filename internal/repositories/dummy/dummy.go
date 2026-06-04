package dummy_repository

import (
	"github.com/jmoiron/sqlx"

	"github.com/Open-Zord/zord/internal/application/domain/dummy"
	"github.com/Open-Zord/zord/internal/repositories/base_repository"
)

// RegistryKey identifica o *DummyRepository no pkg/registry.
const RegistryKey = "dummyRepository"

type DummyRepository struct {
	*base_repository.BaseRepo[dummy.Dummy]
}

func NewDummyRepository(mysql *sqlx.DB) *DummyRepository {
	return &DummyRepository{
		BaseRepo: base_repository.NewBaseRepository[dummy.Dummy](mysql),
	}
}
