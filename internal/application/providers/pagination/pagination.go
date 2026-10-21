package pagination

import (
	"errors"
	"math"

	"github.com/Open-Zord/zord/internal/application/providers/baserepo"
)

// Paginator produz uma página de resultados de um domínio.
type Paginator[Row any] interface {
	PaginationHandler(row Row, page int, limit int) (*Pagination[Row], error)
}

type Pagination[Row any] struct {
	CurrentPage int
	TotalPages  int64
	Data        []Row
}

// Repository é a fatia do repositório que a paginação consome.
type Repository[Row any] interface {
	List(domain Row, limit int, offset int) ([]Row, error)
	Count(domain Row) (int64, error)
}

type Provider[Row baserepo.Domain] struct {
	repo Repository[Row]
}

func NewPaginationProvider[Row baserepo.Domain](repo Repository[Row]) *Provider[Row] {
	return &Provider[Row]{
		repo: repo,
	}
}

func (pp *Provider[Row]) PaginationHandler(row Row, page int, limit int) (*Pagination[Row], error) {
	if limit <= 0 {
		return nil, errors.New("pagination: limit must be positive")
	}

	offset := (page - 1) * limit
	total, err := pp.repo.Count(row)
	if err != nil {
		return nil, errors.New("error on pagination count")
	}

	var data []Row
	if total == 0 {
		return &Pagination[Row]{CurrentPage: page, TotalPages: 0, Data: data}, nil
	}

	totalPages := math.Ceil(float64(total) / float64(limit))
	if page <= int(totalPages) {
		data, err = pp.repo.List(row, limit, offset)
		if err != nil {
			return nil, errors.New("error on data list")
		}
	}

	return &Pagination[Row]{
		CurrentPage: page,
		TotalPages:  int64(totalPages),
		Data:        data,
	}, nil
}
