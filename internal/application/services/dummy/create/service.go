// Package create implementa o use case Create.
package create

import (
	"context"

	"github.com/Open-Zord/zord/internal/application/providers/baseservice"
	"github.com/Open-Zord/zord/pkg/apperror"
)

// RegistryKey identifica o *Service no pkg/registry.
const RegistryKey = "createService"

// Service executa o use case Create.
type Service struct {
	baseservice.BaseService
	response *Response
}

// NewService constrói o Service com suas dependências.
func NewService(logger baseservice.Logger, idCreator baseservice.IdCreator) *Service {
	return &Service{
		BaseService: baseservice.BaseService{Logger: logger, Ulid: idCreator},
	}
}

// Execute roda o use case Create.
func (s *Service) Execute(_ context.Context, request *Request) error {
	if err := request.Validate(); err != nil {
		return apperror.NewInvalid(err.Error())
	}
	s.response = &Response{}
	return nil
}

// GetResponse devolve a resposta produzida pelo Execute.
func (s *Service) GetResponse() (*Response, error) {
	return s.response, nil
}
