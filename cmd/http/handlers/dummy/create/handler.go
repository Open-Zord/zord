// Package create expõe o handler HTTP do use case Create.
package create

import (
	"net/http"

	"github.com/Open-Zord/zord/cmd/http/httperr"
	"github.com/Open-Zord/zord/internal/application/services/dummy/create"
	"github.com/Open-Zord/zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

// RegistryKey identifica o *CreateHandler no pkg/registry.
const RegistryKey = "createHandler"

// CreateHandler atende o use case Create. Mantém as deps já resolvidas pelo New.
type CreateHandler struct {
	svc *create.Service
}

// NewCreateHandler resolve as dependências do handler no registry da aplicação. Falha de resolução quebra Setup() (proposital — falha rápida).
func NewCreateHandler(reg *registry.Registry) *CreateHandler {
	svc := registry.Resolve[*create.Service](reg, create.RegistryKey)
	return &CreateHandler{svc: svc}
}

// Handle executa o use case Create.
func (h *CreateHandler) Handle(c echo.Context) error {
	var data create.Data
	if err := c.Bind(&data); err != nil {
		return httperr.RespondBadRequest(c, err.Error())
	}
	req := create.NewRequest(&data)
	if err := h.svc.Execute(c.Request().Context(), req); err != nil {
		return httperr.Respond(c, err)
	}
	out, _ := h.svc.GetResponse()
	return c.JSON(http.StatusOK, out)
}
