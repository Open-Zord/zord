// Package get expõe o handler HTTP do use case Get.
package get

import (
	"net/http"

	"github.com/Open-Zord/zord/internal/skeleton/runtime/cmd/http/httperr"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/internal/application/services/dummy/get"
	"github.com/Open-Zord/zord/internal/skeleton/runtime/pkg/registry"

	"github.com/labstack/echo/v4"
)

// RegistryKey identifica o *GetHandler no pkg/registry.
const RegistryKey = "getHandler"

// GetHandler atende o use case Get. Mantém as deps já resolvidas pelo New.
type GetHandler struct {
	svc *get.Service
}

// NewGetHandler resolve as dependências do handler no registry da aplicação. Falha de resolução quebra Setup() (proposital — falha rápida).
func NewGetHandler(reg *registry.Registry) *GetHandler {
	svc := registry.Resolve[*get.Service](reg, get.RegistryKey)
	return &GetHandler{svc: svc}
}

// Handle executa o use case Get.
func (h *GetHandler) Handle(c echo.Context) error {
	var data get.Data
	if err := c.Bind(&data); err != nil {
		return httperr.RespondBadRequest(c, err.Error())
	}
	data.ID = c.Param("id")
	req := get.NewRequest(&data)
	out, err := h.svc.Execute(c.Request().Context(), req)
	if err != nil {
		return httperr.Respond(c, err)
	}
	return c.JSON(http.StatusOK, out)
}
