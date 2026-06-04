// Package delete implementa o use case Delete.
package delete

import (
	"context"

	"github.com/Open-Zord/zord/internal/application/services"
)

// RegistryKey identifica o *Service no pkg/registry.
const RegistryKey = "deleteService"

// Service executa o use case Delete.
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

// Execute roda o use case Delete.
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
