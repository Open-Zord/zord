package routes

import (
	"github.com/Open-Zord/zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

type Declarable interface {
	DeclarePrivateRoutes(server *echo.Group, apiPrefix string)
	DeclarePublicRoutes(server *echo.Group, apiPrefix string)
}

func GetRoutes(reg *registry.Registry) map[string]Declarable {
	health := NewHealthRoute()
	dummyListRoutes := NewDummyRoutes(reg)
	return map[string]Declarable{
		"health": health,
		"dummy":  dummyListRoutes,
	}
}
