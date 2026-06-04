// Package create implementa o use case Create.
package create

import (
	"context"

	"github.com/Open-Zord/zord/internal/application/services"
)

// RegistryKey identifica o *Service no pkg/registry.
const RegistryKey = "createService"

// Service executa o use case Create.
type Service struct {
	services.BaseService
	response *Response
}

// NewService constrói o Service com suas dependências.
func NewService(logger services.Logger, idCreator services.IdCreator) *Service {
	return &Service{
		BaseService: services.BaseService{Logger: logger, Ulid: idCreator},
	}
}

// Execute roda o use case Create.
func (s *Service) Execute(_ context.Context, request *Request) error {
	if err := request.Validate(); err != nil {
		return services.NewInvalid(err.Error())
	}
	s.response = &Response{}
	return nil
}

// GetResponse devolve a resposta produzida pelo Execute.
func (s *Service) GetResponse() (*Response, error) {
	return s.response, nil
}
