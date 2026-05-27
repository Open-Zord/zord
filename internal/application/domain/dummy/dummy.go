package dummy

import (
	"github.com/Open-Zord/zord/internal/application/providers/filters"
	"github.com/Open-Zord/zord/internal/application/providers/pagination"
	"github.com/Open-Zord/zord/internal/repositories/base_repository"
)

type Dummy struct {
	ID        string `db:"id"`
	DummyName string `db:"name"`
	Email     string `db:"email"`
	DeletedAt string `db:"deleted_at"`
	client    string
	filters   *filters.Filters
}

func (d *Dummy) SetClient(client string) {
	d.client = client
}

func (d *Dummy) SetFilters(filters *filters.Filters) {
	d.filters = filters
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
	if d.client == "" {
		return "dummy"
	}
	return d.client + "." + "dummy"
}

type Repository interface {
	base_repository.BaseRepository[Dummy]
}

type PaginationProvider interface {
	pagination.IPaginationProvider[Dummy]
}
