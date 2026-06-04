// Package edit expõe o handler HTTP do use case Edit.
package edit

import (
	"net/http"

	"github.com/Open-Zord/zord/cmd/http/httperr"
	"github.com/Open-Zord/zord/internal/application/services/dummy/edit"
	"github.com/Open-Zord/zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

// RegistryKey identifica o *EditHandler no pkg/registry.
const RegistryKey = "editHandler"

// EditHandler atende o use case Edit. Mantém as deps já resolvidas pelo New.
type EditHandler struct {
	svc *edit.Service
}

// NewEditHandler resolve as dependências do handler no registry da aplicação. Falha de resolução quebra Setup() (proposital — falha rápida).
func NewEditHandler(reg *registry.Registry) *EditHandler {
	svc := registry.Resolve[*edit.Service](reg, edit.RegistryKey)
	return &EditHandler{svc: svc}
}

// Handle executa o use case Edit.
func (h *EditHandler) Handle(c echo.Context) error {
	var data edit.Data
	if err := c.Bind(&data); err != nil {
		return httperr.RespondBadRequest(c, err.Error())
	}
	req := edit.NewRequest(&data)
	if err := h.svc.Execute(c.Request().Context(), req); err != nil {
		return httperr.Respond(c, err)
	}
	out, _ := h.svc.GetResponse()
	return c.JSON(http.StatusOK, out)
}
