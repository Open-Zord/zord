// Package delete implementa o use case Delete.
package delete

import (
	"context"

	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/domain/dummy"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/providers/baserepo"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/providers/baseservice"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/apperror"
)

// RegistryKey identifica o *Service no pkg/registry.
const RegistryKey = "deleteService"

// Service executa o use case Delete: remove (soft-delete) um Dummy dentro de
// uma transação (UnitOfWork), propagando o context até o banco.
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

// Execute remove o Dummy pelo id dentro de uma transação. Falha de validação
// vira apperror.NewInvalid; falha de infra vira apperror.NewInternal.
func (s *Service) Execute(ctx context.Context, request *Request) (*Response, error) {
	if err := request.Validate(); err != nil {
		return nil, apperror.NewInvalid(err.Error())
	}

	tx, err := s.uow.Begin(ctx)
	if err != nil {
		return nil, apperror.NewInternal("falha ao iniciar transação").WithCause(err)
	}
	if err := s.repo.Delete(ctx, tx, dummy.Dummy{}, "id", request.Data.ID); err != nil {
		_ = tx.Rollback()
		return nil, apperror.NewInternal("falha ao remover dummy").WithCause(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, apperror.NewInternal("falha ao commitar remoção").WithCause(err)
	}

	return &Response{Deleted: true}, nil
}
