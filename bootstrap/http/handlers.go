package http

import (
	dummycreatehandler "github.com/Open-Zord/zord/cmd/http/handlers/dummy/create"
	dummydeletehandler "github.com/Open-Zord/zord/cmd/http/handlers/dummy/delete"
	dummyedithandler "github.com/Open-Zord/zord/cmd/http/handlers/dummy/edit"
	dummygethandler "github.com/Open-Zord/zord/cmd/http/handlers/dummy/get"
	dummylisthandler "github.com/Open-Zord/zord/cmd/http/handlers/dummy/list"
	"github.com/Open-Zord/zord/pkg/registry"
)

// registerHandlers builds and registers the HTTP handlers into the registry.
// Each handler resolves its service(s) from the registry in its constructor, so
// a missing dependency fails fast at boot. The scaffold tool appends new
// handler registrations here.
func registerHandlers(reg *registry.Registry) {
	reg.Provide(dummycreatehandler.RegistryKey, dummycreatehandler.NewCreateHandler(reg))
	reg.Provide(dummygethandler.RegistryKey, dummygethandler.NewGetHandler(reg))
	reg.Provide(dummylisthandler.RegistryKey, dummylisthandler.NewListHandler(reg))
	reg.Provide(dummyedithandler.RegistryKey, dummyedithandler.NewEditHandler(reg))
	reg.Provide(dummydeletehandler.RegistryKey, dummydeletehandler.NewDeleteHandler(reg))
}
