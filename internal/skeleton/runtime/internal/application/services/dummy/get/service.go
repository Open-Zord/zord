// Package get implementa o use case Get.
package get

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/domain/dummy"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/providers/baseservice"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/apperror"
)

// RegistryKey identifica o *Service no pkg/registry.
const RegistryKey = "getService"

// Service executa o use case Get: lê um Dummy pelo id. Leitura pura, sem
// transação — resolve o repositório por injeção e propaga o context.
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

// Execute busca o Dummy pelo id. Não encontrado vira apperror.NewNotFound;
// falha de infra vira apperror.NewInternal com a causa anexada.
func (s *Service) Execute(ctx context.Context, request *Request) (*Response, error) {
	if err := request.Validate(); err != nil {
		return nil, apperror.NewInvalid(err.Error())
	}

	record, err := s.repo.Get(ctx, dummy.Dummy{}, "id", request.Data.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.NewNotFound("dummy não encontrado")
		}
		return nil, apperror.NewInternal("falha ao buscar dummy").WithCause(err)
	}

	return &Response{ID: record.ID, Name: record.Name, Email: record.Email}, nil
}
