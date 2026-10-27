// Package edit implementa o use case Edit.
package edit

import (
	"context"

	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/domain/dummy"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/providers/baserepo"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/providers/baseservice"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/apperror"
)

// RegistryKey identifica o *Service no pkg/registry.
const RegistryKey = "editService"

// Service executa o use case Edit: atualiza um Dummy dentro de uma transação
// (UnitOfWork), propagando o context até o banco.
type Service struct {
	baseservice.BaseService
	repo dummy.Repository
	uow  baserepo.UnitOfWork
}

// NewService constrói o Service com suas dependências.
func NewService(logger baseservice.Logger, idCreator baseservice.IDCreator, repo dummy.Repository, uow baserepo.UnitOfWork) *Service {
	return &Service{
		BaseService: baseservice.BaseService{Logger: logger, ULID: idCreator},
		repo:        repo,
		uow:         uow,
	}
}

// Execute atualiza o Dummy pelo id dentro de uma transação. Falha de validação
// vira apperror.NewInvalid; falha de infra vira apperror.NewInternal.
func (s *Service) Execute(ctx context.Context, request *Request) (*Response, error) {
	if err := request.Validate(); err != nil {
		return nil, apperror.NewInvalid(err.Error())
	}

	record := dummy.Dummy{
		ID:    request.Data.ID,
		Name:  request.Data.Name,
		Email: request.Data.Email,
	}

	tx, err := s.uow.Begin(ctx)
	if err != nil {
		return nil, apperror.NewInternal("falha ao iniciar transação").WithCause(err)
	}
	affected, err := s.repo.Edit(ctx, tx, record, "id", request.Data.ID)
	if err != nil {
		_ = tx.Rollback()
		return nil, apperror.NewInternal("falha ao editar dummy").WithCause(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, apperror.NewInternal("falha ao commitar edição").WithCause(err)
	}

	return &Response{Updated: affected}, nil
}
