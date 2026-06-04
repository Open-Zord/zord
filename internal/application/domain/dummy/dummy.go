package dummy

import (
	"github.com/Open-Zord/zord/internal/application/domain/baserepo"
)

type Dummy struct {
	ID    string `db:"id" db_type:"char(26)" db_pk:""`
	Name  string `db:"name" json:"name" validate:"required" db_type:"char(255)"`
	Email string `db:"email" json:"email" validate:"required,email" db_type:"char(255)"`
}

func (d Dummy) SoftDelete() string {
	return "deleted_at"
}

func (d Dummy) Schema() string {
	return "dummys"
}

type Repository interface {
	baserepo.Repository[Dummy]
}
