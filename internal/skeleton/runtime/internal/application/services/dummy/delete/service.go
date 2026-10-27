// Package delete implementa o use case Delete.
package delete

import (
	"context"

	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/providers/baseservice"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/apperror"
)

// RegistryKey identifica o *Service no pkg/registry.
const RegistryKey = "deleteService"

// Service executa o use case Delete.
type Service struct {
	baseservice.BaseService
}

// NewService constrói o Service com suas dependências.
func NewService(logger baseservice.Logger, idCreator baseservice.IDCreator) *Service {
	return &Service{
		BaseService: baseservice.BaseService{Logger: logger, ULID: idCreator},
	}
}

// Execute roda o use case Delete.
func (s *Service) Execute(_ context.Context, request *Request) (*Response, error) {
	if err := request.Validate(); err != nil {
		return nil, apperror.NewInvalid(err.Error())
	}
	return &Response{}, nil
}
