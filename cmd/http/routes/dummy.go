package routes

import (
	"github.com/Open-Zord/zord/cmd/http/handlers/dummy/create"
	"github.com/Open-Zord/zord/cmd/http/handlers/dummy/delete"
	"github.com/Open-Zord/zord/cmd/http/handlers/dummy/edit"
	"github.com/Open-Zord/zord/cmd/http/handlers/dummy/get"
	"github.com/Open-Zord/zord/cmd/http/handlers/dummy/list"
	"github.com/Open-Zord/zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

type DummyRoute struct {
	createHandler *create.CreateHandler
	getHandler    *get.GetHandler
	listHandler   *list.ListHandler
	editHandler   *edit.EditHandler
	deleteHandler *delete.DeleteHandler
}

func NewDummyRoute(reg *registry.Registry) *DummyRoute {
	return &DummyRoute{
		createHandler: registry.Resolve[*create.CreateHandler](reg, create.RegistryKey),
		getHandler:    registry.Resolve[*get.GetHandler](reg, get.RegistryKey),
		listHandler:   registry.Resolve[*list.ListHandler](reg, list.RegistryKey),
		editHandler:   registry.Resolve[*edit.EditHandler](reg, edit.RegistryKey),
		deleteHandler: registry.Resolve[*delete.DeleteHandler](reg, delete.RegistryKey),
	}
}

func (r *DummyRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {
	g.POST("/"+prefix+"/dummy"+"/", r.createHandler.Handle)
	g.GET("/"+prefix+"/dummy"+"/:id", r.getHandler.Handle)
	g.GET("/"+prefix+"/dummy"+"/", r.listHandler.Handle)
	g.PUT("/"+prefix+"/dummy"+"/:id", r.editHandler.Handle)
	g.DELETE("/"+prefix+"/dummy"+"/:id", r.deleteHandler.Handle)
}

func (r *DummyRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {
}
