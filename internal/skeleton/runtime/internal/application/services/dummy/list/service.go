// Package list implementa o use case List.
package list

import (
	"context"

	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/domain/dummy"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/providers/baseservice"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/apperror"
)

// RegistryKey identifica o *Service no pkg/registry.
const RegistryKey = "listService"

// defaultLimit é o tamanho de página usado quando o request não informa Limit.
const defaultLimit = 20

// Service executa o use case List: lista Dummies paginados. Leitura pura.
type Service struct {
	baseservice.BaseService
	repo dummy.Repository
}

// NewService constrói o Service com suas dependências.
func NewService(logger baseservice.Logger, idCreator baseservice.IDCreator, repo dummy.Repository) *Service {
	return &Service{
		BaseService: baseservice.BaseService{Logger: logger, ULID: idCreator},
		repo:        repo,
	}
}

// Execute lista Dummies com LIMIT/OFFSET (defaults saneados). Falha de infra
// vira apperror.NewInternal com a causa anexada.
func (s *Service) Execute(ctx context.Context, request *Request) (*Response, error) {
	if err := request.Validate(); err != nil {
		return nil, apperror.NewInvalid(err.Error())
	}

	limit := defaultLimit
	offset := 0
	if request.Data != nil {
		if request.Data.Limit > 0 {
			limit = request.Data.Limit
		}
		if request.Data.Offset > 0 {
			offset = request.Data.Offset
		}
	}

	records, err := s.repo.List(ctx, dummy.Dummy{}, limit, offset)
	if err != nil {
		return nil, apperror.NewInternal("falha ao listar dummies").WithCause(err)
	}

	items := make([]Item, 0, len(records))
	for _, r := range records {
		items = append(items, Item{ID: r.ID, Name: r.Name, Email: r.Email})
	}
	return &Response{Items: items}, nil
}
