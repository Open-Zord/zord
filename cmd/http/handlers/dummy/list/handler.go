// Package list expõe o handler HTTP do use case List.
package list

import (
	"net/http"

	"github.com/Open-Zord/zord/cmd/http/httperr"
	"github.com/Open-Zord/zord/internal/application/services/dummy/list"
	"github.com/Open-Zord/zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

// RegistryKey identifica o *ListHandler no pkg/registry.
const RegistryKey = "listHandler"

// ListHandler atende o use case List. Mantém as deps já resolvidas pelo New.
type ListHandler struct {
	svc *list.Service
}

// NewListHandler resolve as dependências do handler no registry da aplicação. Falha de resolução quebra Setup() (proposital — falha rápida).
func NewListHandler(reg *registry.Registry) *ListHandler {
	svc := registry.Resolve[*list.Service](reg, list.RegistryKey)
	return &ListHandler{svc: svc}
}

// Handle executa o use case List.
func (h *ListHandler) Handle(c echo.Context) error {
	var data list.Data
	if err := c.Bind(&data); err != nil {
		return httperr.RespondBadRequest(c, err.Error())
	}
	req := list.NewRequest(&data)
	if err := h.svc.Execute(c.Request().Context(), req); err != nil {
		return httperr.Respond(c, err)
	}
	out, _ := h.svc.GetResponse()
	return c.JSON(http.StatusOK, out)
}
