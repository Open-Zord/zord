// Package create implementa o use case Create.
package create

import (
	"context"

	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/domain/dummy"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/providers/baserepo"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/providers/baseservice"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/apperror"
)

// RegistryKey identifica o *Service no pkg/registry.
const RegistryKey = "createService"

// Service executa o use case Create. Exemplo de referência do fluxo completo:
// recebe o repositório e a UnitOfWork por injeção, persiste dentro de uma
// transação e propaga o context até o banco.
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

// Execute cria um Dummy dentro de uma transação (UnitOfWork), propagando ctx
// até o banco. Falha de validação vira apperror.NewInvalid; falha de infra vira
// apperror.NewInternal com a causa anexada (logada no transporte, oculta ao
// cliente).
func (s *Service) Execute(ctx context.Context, request *Request) (*Response, error) {
	if err := request.Validate(); err != nil {
		return nil, apperror.NewInvalid(err.Error())
	}

	record := dummy.Dummy{
		ID:    s.ULID.Create(),
		Name:  request.Data.Name,
		Email: request.Data.Email,
	}

	tx, err := s.uow.Begin(ctx)
	if err != nil {
		return nil, apperror.NewInternal("falha ao iniciar transação").WithCause(err)
	}
	if err := s.repo.Create(ctx, tx, record); err != nil {
		_ = tx.Rollback()
		return nil, apperror.NewInternal("falha ao criar dummy").WithCause(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, apperror.NewInternal("falha ao commitar dummy").WithCause(err)
	}

	return &Response{ID: record.ID}, nil
}
