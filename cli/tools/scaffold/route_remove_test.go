package scaffold

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedAddedRoute roda a cadeia completa até route add, deixando o root
// pronto pra um route remove.
func seedAddedRoute(t *testing.T, root, dom, svc, method string, public bool) {
	t.Helper()
	seedAll(t, root, dom, svc)
	if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: dom, Service: svc, Method: method, Public: public}); err != nil {
		t.Fatalf("seed RouteAdd %s/%s: %v", dom, svc, err)
	}
}

func TestRouteRemove_HappyPath_Private(t *testing.T) {
	root := t.TempDir()
	seedAddedRoute(t, root, "Auth", "Login", "POST", false)

	rel, err := RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err != nil {
		t.Fatalf("RouteRemove: %v", err)
	}
	if want := filepath.Join("cmd/http/routes/auth.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustNotContain(t, got,
		`"zord/cmd/http/handlers/auth/login"`,
		"loginHandler",
		"login.LoginHandler",
		"r.loginHandler.Handle",
	)
	// Estrutura canônica preservada.
	mustContain(t, got,
		"type AuthRoute struct {",
		"func NewAuthRoute(reg *registry.Registry) *AuthRoute {",
		"return &AuthRoute{}",
		"func (r *AuthRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {",
		"func (r *AuthRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {",
	)
	if _, err := parser.ParseFile(token.NewFileSet(), "", got, parser.SkipObjectResolution); err != nil {
		t.Fatalf("arquivo final não parseia: %v\n%s", err, got)
	}
}

func TestRouteRemove_HappyPath_Public(t *testing.T) {
	root := t.TempDir()
	seedAddedRoute(t, root, "Auth", "Login", "POST", true)

	if _, err := RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("RouteRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, "cmd/http/routes/auth.go"))
	mustNotContain(t, got,
		`"zord/cmd/http/handlers/auth/login"`,
		"loginHandler",
		"r.loginHandler.Handle",
	)
}

func TestRouteRemove_FailsIfRouteAbsent(t *testing.T) {
	root := t.TempDir()
	// route create mas sem nenhum route add — campo/KV/stmt/import ausentes
	seedAll(t, root, "Auth", "Login")

	_, err := RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("RouteRemove: esperado erro pra rota ausente")
	}
	if !strings.Contains(err.Error(), "loginHandler") {
		t.Errorf("erro %q não menciona o nome do campo esperado", err.Error())
	}
}

func TestRouteRemove_FailsIfCtorHandEdited(t *testing.T) {
	// Ctor com assinatura não canônica deve falhar — mesmo sem --force, e o
	// teste abaixo cobre que também falha com --force (regra explícita).
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")
	// quebra a assinatura do ctor.
	hand := `package routes

import (
	"zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

var _ *registry.Registry
var _ *echo.Group

type AuthRoute struct {
}

func NewAuthRoute(reg *registry.Registry, extra string) *AuthRoute {
	return &AuthRoute{}
}

func (r *AuthRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {
}

func (r *AuthRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {
}
`
	if err := os.WriteFile(filepath.Join(root, "cmd/http/routes/auth.go"), []byte(hand), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	_, err := RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil || !strings.Contains(err.Error(), "assinatura canônica") {
		t.Fatalf("RouteRemove: esperado erro mencionando 'assinatura canônica', got: %v", err)
	}
	// Mesma falha mesmo com --force.
	_, err = RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login", Force: true})
	if err == nil || !strings.Contains(err.Error(), "assinatura canônica") {
		t.Fatalf("RouteRemove --force: esperado mesmo erro fatal de ctor, got: %v", err)
	}
}

func TestRouteRemove_LastHandlerEmptiesStructAndDropsImport(t *testing.T) {
	root := t.TempDir()
	seedAddedRoute(t, root, "Auth", "Login", "POST", false)

	if _, err := RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("RouteRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, "cmd/http/routes/auth.go"))
	// Struct esvaziada — gofmt colapsa pra `&AuthRoute{}` em linha única.
	mustContain(t, got, "return &AuthRoute{}")
	// Import do handler some.
	mustNotContain(t, got, `"zord/cmd/http/handlers/auth/login"`)
	// `pkg/registry` permanece — ctor ainda assina com ele.
	mustContain(t, got, `"zord/pkg/registry"`)
}

func TestRouteRemove_ForceTolerantesPartialState(t *testing.T) {
	// Cenário: hand-edit removeu o campo da struct, mas a atribuição do
	// ctor + ExprStmt + import permanecem. Sem --force, falha; com --force,
	// limpa o resíduo.
	root := t.TempDir()
	seedAddedRoute(t, root, "Auth", "Login", "POST", false)
	authPath := filepath.Join(root, "cmd/http/routes/auth.go")
	before := readFile(t, authPath)
	// Remove só a linha do campo da struct.
	stripped := strings.Replace(before, "loginHandler *login.LoginHandler", "", 1)
	if stripped == before {
		t.Fatalf("setup do teste não removeu o campo esperado:\n%s", before)
	}
	if err := os.WriteFile(authPath, []byte(stripped), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	// Sem --force: falha por campo ausente.
	if _, err := RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login"}); err == nil {
		t.Fatalf("RouteRemove sem --force: esperado erro pra campo ausente")
	}
	// Com --force: aplica nos demais.
	if _, err := RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login", Force: true}); err != nil {
		t.Fatalf("RouteRemove --force: %v", err)
	}
	got := readFile(t, authPath)
	mustNotContain(t, got,
		`"zord/cmd/http/handlers/auth/login"`,
		"loginHandler:",
		"r.loginHandler.Handle",
	)
}

func TestRouteRemove_PreservesImportWhenOtherRouteReferencesService(t *testing.T) {
	// Hand-edit: arquivo tem 2 rotas (POST + GET) referenciando o mesmo
	// service via mesmo pkg ident (`login.RegistryKey` em dois KVs). Após
	// remover uma, o import deve sobreviver porque a outra ainda usa
	// `login.<X>`.
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")
	hand := `package routes

import (
	"zord/cmd/http/handlers/auth/login"
	"zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

type AuthRoute struct {
	loginHandler *login.LoginHandler
	loginAltHandler *login.LoginHandler
}

func NewAuthRoute(reg *registry.Registry) *AuthRoute {
	return &AuthRoute{
		loginHandler: registry.Resolve[*login.LoginHandler](reg, login.RegistryKey),
		loginAltHandler: registry.Resolve[*login.LoginHandler](reg, login.RegistryKey),
	}
}

func (r *AuthRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {
	g.POST("/"+prefix+"/auth"+"/login", r.loginHandler.Handle)
	g.GET("/"+prefix+"/auth"+"/login-alt", r.loginAltHandler.Handle)
}

func (r *AuthRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {
}
`
	if err := os.WriteFile(filepath.Join(root, "cmd/http/routes/auth.go"), []byte(hand), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	// Remove só Login. LoginAlt continua usando o mesmo pacote — import
	// não deve sumir.
	if _, err := RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("RouteRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, "cmd/http/routes/auth.go"))
	mustContain(t, got,
		`"zord/cmd/http/handlers/auth/login"`,
		"loginAltHandler *login.LoginHandler",
		"r.loginAltHandler.Handle",
	)
	mustNotContain(t, got,
		"loginHandler *login.LoginHandler",
		"r.loginHandler.Handle",
	)
	if _, err := parser.ParseFile(token.NewFileSet(), "", got, parser.SkipObjectResolution); err != nil {
		t.Fatalf("arquivo final não parseia: %v\n%s", err, got)
	}
}

func TestRouteRemove_NegativeIdempotence(t *testing.T) {
	// Re-rodar após sucesso falha porque campo/KV/stmt/import já não existem.
	root := t.TempDir()
	seedAddedRoute(t, root, "Auth", "Login", "POST", false)
	if _, err := RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login"}); err != nil {
		t.Fatalf("primeiro RouteRemove: %v", err)
	}
	_, err := RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("segundo RouteRemove: esperado erro (idempotência negativa), got nil")
	}
}

func TestRouteRemove_FailsIfDomainInvalid(t *testing.T) {
	if _, err := RouteRemove(RouteRemoveOptions{Domain: "lowercase", Service: "Login"}); err == nil {
		t.Fatalf("RouteRemove: esperado erro pra Domain inválido")
	}
}

func TestRouteRemove_FailsIfServiceInvalid(t *testing.T) {
	if _, err := RouteRemove(RouteRemoveOptions{Domain: "Auth", Service: "lowercase"}); err == nil {
		t.Fatalf("RouteRemove: esperado erro pra Service inválido")
	}
}

func TestRouteRemove_FailsIfRouteFileMissing(t *testing.T) {
	root := t.TempDir()
	// nem route create
	_, err := RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("RouteRemove: esperado erro pra route file ausente")
	}
}

func TestRouteRemove_FailsIfRouteFileUnparseable(t *testing.T) {
	root := t.TempDir()
	seedAddedRoute(t, root, "Auth", "Login", "POST", false)
	if err := os.WriteFile(filepath.Join(root, "cmd/http/routes/auth.go"), []byte("not go code"), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if _, err := RouteRemove(RouteRemoveOptions{Root: root, Domain: "Auth", Service: "Login"}); err == nil {
		t.Fatalf("RouteRemove: esperado erro pra arquivo unparseable")
	}
}
