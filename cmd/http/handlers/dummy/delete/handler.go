// Package delete expõe o handler HTTP do use case Delete.
package delete

import (
	"net/http"

	"github.com/Open-Zord/zord/cmd/http/httperr"
	"github.com/Open-Zord/zord/internal/application/services/dummy/delete"
	"github.com/Open-Zord/zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

// RegistryKey identifica o *DeleteHandler no pkg/registry.
const RegistryKey = "deleteHandler"

// DeleteHandler atende o use case Delete. Mantém as deps já resolvidas pelo New.
type DeleteHandler struct {
	svc *delete.Service
}

// NewDeleteHandler resolve as dependências do handler no registry da aplicação. Falha de resolução quebra Setup() (proposital — falha rápida).
func NewDeleteHandler(reg *registry.Registry) *DeleteHandler {
	svc := registry.Resolve[*delete.Service](reg, delete.RegistryKey)
	return &DeleteHandler{svc: svc}
}

// Handle executa o use case Delete.
func (h *DeleteHandler) Handle(c echo.Context) error {
	var data delete.Data
	if err := c.Bind(&data); err != nil {
		return httperr.RespondBadRequest(c, err.Error())
	}
	req := delete.NewRequest(&data)
	if err := h.svc.Execute(c.Request().Context(), req); err != nil {
		return httperr.Respond(c, err)
	}
	out, _ := h.svc.GetResponse()
	return c.JSON(http.StatusOK, out)
}
