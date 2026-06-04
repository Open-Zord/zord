package dummy

import (
	"github.com/Open-Zord/zord/internal/application/providers/filters"
	"github.com/Open-Zord/zord/internal/repositories/base_repository"
)

type Dummy struct {
	ID      string `db:"id" db_type:"char(26)" db_pk:""`
	Name    string `db:"name" json:"name" validate:"required" db_type:"char(255)"`
	Email   string `db:"email" json:"email" validate:"required,email" db_type:"char(255)"`
	filters *filters.Filters
}

func (d *Dummy) SetFilters(f *filters.Filters) {
	d.filters = f
}

func (d Dummy) SoftDelete() string {
	return "deleted_at"
}

func (d Dummy) GetFilters() filters.Filters {
	if d.filters != nil {
		return *d.filters
	}
	return filters.Filters{}
}

func (d Dummy) Schema() string {
	return "dummys"
}

type Repository interface {
	base_repository.BaseRepository[Dummy]
}
