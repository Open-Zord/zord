package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedHandlerWithoutBootstrap seeda domain + service + handler, sem bootstrap.
// Cobre o cenário em que `handler delete` é o primeiro comando rodado num
// repo que ainda não tem wire-up.
func seedHandlerWithoutBootstrap(t *testing.T, root, dom, service string) {
	t.Helper()
	seedDomain(t, root, dom)
	seedService(t, root, dom, service)
	seedHandler(t, root, dom, service)
}

func TestHandlerDelete_HappyPath_BootstrapAbsent(t *testing.T) {
	root := t.TempDir()
	seedHandlerWithoutBootstrap(t, root, "Auth", "Login")

	rel, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err != nil {
		t.Fatalf("HandlerDelete: %v", err)
	}
	want := filepath.Join(handlersBasePath, "auth", "login")
	if rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	if _, err := os.Stat(filepath.Join(root, want)); !os.IsNotExist(err) {
		t.Errorf("pasta ainda existe após delete: err=%v", err)
	}
}

func TestHandlerDelete_HappyPath_BootstrapPresentNoWireUp(t *testing.T) {
	root := t.TempDir()
	seedHandlerWithoutBootstrap(t, root, "Auth", "Login")
	seedBootstrapHandlers(t, root)

	rel, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err != nil {
		t.Fatalf("HandlerDelete: %v", err)
	}
	if rel != filepath.Join(handlersBasePath, "auth", "login") {
		t.Errorf("path inesperado: %q", rel)
	}
	if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
		t.Errorf("pasta ainda existe após delete")
	}
	// Bootstrap intacto.
	got := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))
	mustContain(t, got, "func registerHandlers(reg *registry.Registry)")
}

func TestHandlerDelete_HappyPath_BootstrapSemRegisterFunc(t *testing.T) {
	root := t.TempDir()
	seedHandlerWithoutBootstrap(t, root, "Auth", "Login")
	// Bootstrap presente mas sem `registerHandlers` — não há wire-up possível.
	absFile := filepath.Join(root, "bootstrap", "handlers.go")
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package bootstrap

import "zord/pkg/registry"

func registerOther(reg *registry.Registry) {
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("HandlerDelete: %v", err)
	}
}

func TestHandlerDelete_HappyPath_RouteFilePresentSemUso(t *testing.T) {
	// Route file presente mas sem campo/import/uso do handler — passa.
	root := t.TempDir()
	seedHandlerWithoutBootstrap(t, root, "Auth", "Login")
	if _, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Auth"}); err != nil {
		t.Fatalf("RouteCreate: %v", err)
	}

	rel, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err != nil {
		t.Fatalf("HandlerDelete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
		t.Errorf("pasta ainda existe após delete")
	}
	// Route file intacto.
	got := readFile(t, filepath.Join(root, routesBasePath, "auth.go"))
	mustContain(t, got, "type AuthRoute struct")
}

func TestHandlerDelete_FailsIfHandlerMissing(t *testing.T) {
	root := t.TempDir()
	// Nada criado.

	_, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("esperado erro pra handler ausente, got nil")
	}
	if !strings.Contains(err.Error(), "não existe") {
		t.Errorf("erro %q não menciona 'não existe'", err.Error())
	}
}

func TestHandlerDelete_FailsIfImportStillPresent(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")
	seedBootstrapHandlers(t, root)
	if _, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("RegisterHandler: %v", err)
	}

	before := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))
	_, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("esperado erro pra wire-up vivo, got nil")
	}
	if !strings.Contains(err.Error(), "import") || !strings.Contains(err.Error(), "scaffold handler unregister") {
		t.Errorf("erro %q não orienta a rodar unregister", err.Error())
	}
	// Pasta intocada.
	if _, statErr := os.Stat(filepath.Join(root, handlersBasePath, "auth", "login")); statErr != nil {
		t.Errorf("pasta foi apagada apesar do erro: %v", statErr)
	}
	// Bootstrap intocado.
	after := readFile(t, filepath.Join(root, "bootstrap", "handlers.go"))
	if before != after {
		t.Errorf("bootstrap mutado em falha:\n%s", after)
	}
}

func TestHandlerDelete_FailsIfProvideStillPresentWithoutImport(t *testing.T) {
	// Caso patológico: arquivo com Provide mas sem o import equivalente.
	// handler delete precisa rejeitar mesmo assim.
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")
	absFile := filepath.Join(root, "bootstrap", "handlers.go")
	if err := os.MkdirAll(filepath.Dir(absFile), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Alias canônico (NAVE-70): "auth" + "login" + "handler" = "authloginhandler"
	src := `package bootstrap

import "zord/pkg/registry"

func registerHandlers(reg *registry.Registry) {
	reg.Provide(authloginhandler.RegistryKey, authloginhandler.NewLoginHandler(reg))
}
`
	if err := os.WriteFile(absFile, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("esperado erro pra Provide vivo sem import, got nil")
	}
	if !strings.Contains(err.Error(), "Provide") {
		t.Errorf("erro %q não menciona Provide", err.Error())
	}
}

func TestHandlerDelete_FailsIfRouteFieldStillPresent(t *testing.T) {
	root := t.TempDir()
	seedHandlerWithoutBootstrap(t, root, "Auth", "Login")
	if _, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Auth"}); err != nil {
		t.Fatalf("RouteCreate: %v", err)
	}
	if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"}); err != nil {
		t.Fatalf("RouteAdd: %v", err)
	}

	_, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("esperado erro pra rota viva, got nil")
	}
	// Guarda primária da rota é o campo (vem antes do import na struct, então
	// o erro deve mencionar o campo). Aceita também "import" caso a ordem do
	// AST visitor entregue a struct depois (defensivo).
	if !strings.Contains(err.Error(), "loginHandler") && !strings.Contains(err.Error(), "import") {
		t.Errorf("erro %q não menciona o campo loginHandler nem import", err.Error())
	}
	// Pasta intocada.
	if _, statErr := os.Stat(filepath.Join(root, handlersBasePath, "auth", "login")); statErr != nil {
		t.Errorf("pasta foi apagada apesar do erro: %v", statErr)
	}
}

func TestHandlerDelete_FailsIfRouteImportStillPresentWithoutField(t *testing.T) {
	// Caso patológico: route file com import do handler mas sem campo nem uso.
	// handler delete precisa rejeitar mesmo assim.
	root := t.TempDir()
	seedHandlerWithoutBootstrap(t, root, "Auth", "Login")
	absRoute := filepath.Join(root, routesBasePath, "auth.go")
	if err := os.MkdirAll(filepath.Dir(absRoute), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package routes

import (
	_ "zord/cmd/http/handlers/auth/login"
	"zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

type AuthRoute struct{}

func NewAuthRoute(reg *registry.Registry) *AuthRoute { return &AuthRoute{} }

func (r *AuthRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {}
func (r *AuthRoute) DeclarePublicRoutes(g *echo.Group, prefix string)  {}
`
	if err := os.WriteFile(absRoute, []byte(src), 0o600); err != nil {
		t.Fatalf("write route: %v", err)
	}

	_, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("esperado erro pra import vivo na rota, got nil")
	}
	if !strings.Contains(err.Error(), "import") {
		t.Errorf("erro %q não menciona import", err.Error())
	}
}

func TestHandlerDelete_FailsIfRouteHandleCallStillPresent(t *testing.T) {
	// Caso patológico: route file sem campo nem import, mas com chamada
	// r.<lowerCamel>Handler.Handle hand-editada. handler delete rejeita.
	root := t.TempDir()
	seedHandlerWithoutBootstrap(t, root, "Auth", "Login")
	absRoute := filepath.Join(root, routesBasePath, "auth.go")
	if err := os.MkdirAll(filepath.Dir(absRoute), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package routes

import (
	"zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

type AuthRoute struct{}

func NewAuthRoute(reg *registry.Registry) *AuthRoute { return &AuthRoute{} }

func (r *AuthRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {
	g.POST("/login", r.loginHandler.Handle)
}
func (r *AuthRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {}
`
	if err := os.WriteFile(absRoute, []byte(src), 0o600); err != nil {
		t.Fatalf("write route: %v", err)
	}

	_, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("esperado erro pra Handle vivo, got nil")
	}
	if !strings.Contains(err.Error(), "Handle") {
		t.Errorf("erro %q não menciona Handle", err.Error())
	}
}

func TestHandlerDelete_FailsOnInvalidIdent(t *testing.T) {
	cases := []struct {
		name    string
		domain  string
		service string
	}{
		{"domain lowercase", "auth", "Login"},
		{"domain empty", "", "Login"},
		{"domain non-ident", "Auth-Org", "Login"},
		{"service lowercase", "Auth", "login"},
		{"service empty", "Auth", ""},
		{"service non-ident", "Auth", "Login-Now"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := HandlerDelete(HandlerDeleteOptions{Domain: tc.domain, Service: tc.service})
			if err == nil {
				t.Fatalf("HandlerDelete(%q,%q): esperado erro, got nil", tc.domain, tc.service)
			}
		})
	}
}

func TestHandlerDelete_IsIdempotentFailureAfterSuccess(t *testing.T) {
	root := t.TempDir()
	seedHandlerWithoutBootstrap(t, root, "Auth", "Login")

	if _, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("primeiro HandlerDelete: %v", err)
	}
	_, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("segundo HandlerDelete: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "não existe") {
		t.Errorf("erro idempotente %q não menciona 'não existe'", err.Error())
	}
}

func TestHandlerDelete_FullCycleAfterUnregister(t *testing.T) {
	// Fluxo end-to-end: create handler → register → unregister → delete deve passar.
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")
	seedBootstrapHandlers(t, root)
	if _, err := RegisterHandler(RegisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("RegisterHandler: %v", err)
	}
	if _, err := UnregisterHandler(UnregisterHandlerOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("UnregisterHandler: %v", err)
	}

	rel, err := HandlerDelete(HandlerDeleteOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err != nil {
		t.Fatalf("HandlerDelete após unregister: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(statErr) {
		t.Errorf("pasta ainda existe: %v", statErr)
	}
}
