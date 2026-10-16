package dummyrepo

import (
	"github.com/jmoiron/sqlx"

	"github.com/Open-Zord/zord/internal/application/domain/dummy"
	"github.com/Open-Zord/zord/internal/repositories/baserepository"
)

// RegistryKey identifica o *DummyRepository no pkg/registry.
const RegistryKey = "dummyRepository"

type DummyRepository struct {
	*baserepository.BaseRepo[dummy.Dummy]
}

func NewDummyRepository(mysql *sqlx.DB) *DummyRepository {
	return &DummyRepository{
		BaseRepo: baserepository.NewBaseRepository[dummy.Dummy](mysql),
	}
}
