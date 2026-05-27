package scaffold

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouteAdd_HappyPath_Private(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")

	rel, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
	if err != nil {
		t.Fatalf("RouteAdd: %v", err)
	}
	if want := filepath.Join("cmd/http/routes/auth.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		`"zord/cmd/http/handlers/auth/login"`,
		`"zord/pkg/registry"`,
		`"github.com/labstack/echo/v4"`,
		"loginHandler *login.LoginHandler",
		"func NewAuthRoute(reg *registry.Registry) *AuthRoute {",
		"loginHandler: registry.Resolve[*login.LoginHandler](reg, login.RegistryKey)",
		`g.POST("/"+prefix+"/auth"+"/login", r.loginHandler.Handle)`,
	)
	mustNotContain(t, got, "DeclarePublicRoutes(g *echo.Group, prefix string) {\n\tg.")
}

func TestRouteAdd_HappyPath_Public(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")

	if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST", Public: true}); err != nil {
		t.Fatalf("RouteAdd: %v", err)
	}
	got := readFile(t, filepath.Join(root, "cmd/http/routes/auth.go"))
	mustContain(t, got,
		"func (r *AuthRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {",
		`g.POST("/"+prefix+"/auth"+"/login", r.loginHandler.Handle)`,
	)
	// Private deve continuar vazio
	mustNotContain(t, got, "DeclarePrivateRoutes(g *echo.Group, prefix string) {\n\tg.")
}

func TestRouteAdd_HappyPath_CustomPath(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Org", "GetOrg")

	if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Org", Service: "GetOrg", Method: "GET", Path: "/"}); err != nil {
		t.Fatalf("RouteAdd: %v", err)
	}
	got := readFile(t, filepath.Join(root, "cmd/http/routes/org.go"))
	mustContain(t, got,
		`g.GET("/"+prefix+"/org"+"/", r.getOrgHandler.Handle)`,
	)
	// Sem barra dupla.
	if strings.Contains(got, `"//"`) {
		t.Errorf("path contém barra dupla literal:\n%s", got)
	}
}

func TestRouteAdd_HappyPath_CompoundService(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Auth", "SelectOrg")

	if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "SelectOrg", Method: "POST"}); err != nil {
		t.Fatalf("RouteAdd: %v", err)
	}
	got := readFile(t, filepath.Join(root, "cmd/http/routes/auth.go"))
	mustContain(t, got,
		`"zord/cmd/http/handlers/auth/select_org"`,
		`"zord/pkg/registry"`,
		"selectOrgHandler *select_org.SelectOrgHandler",
		"func NewAuthRoute(reg *registry.Registry) *AuthRoute {",
		"selectOrgHandler: registry.Resolve[*select_org.SelectOrgHandler](reg, select_org.RegistryKey)",
		`g.POST("/"+prefix+"/auth"+"/select-org", r.selectOrgHandler.Handle)`,
	)
}

func TestRouteAdd_HappyPath_Accumulative(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	if _, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Auth"}); err != nil {
		t.Fatalf("RouteCreate: %v", err)
	}
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")
	seedService(t, root, "Auth", "Register")
	seedHandler(t, root, "Auth", "Register")
	seedService(t, root, "Auth", "SelectOrg")
	seedHandler(t, root, "Auth", "SelectOrg")

	if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST", Public: true}); err != nil {
		t.Fatalf("RouteAdd Login: %v", err)
	}
	if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Register", Method: "POST", Public: true}); err != nil {
		t.Fatalf("RouteAdd Register: %v", err)
	}
	if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "SelectOrg", Method: "POST"}); err != nil {
		t.Fatalf("RouteAdd SelectOrg: %v", err)
	}

	got := readFile(t, filepath.Join(root, "cmd/http/routes/auth.go"))
	mustContain(t, got,
		`"zord/cmd/http/handlers/auth/login"`,
		`"zord/cmd/http/handlers/auth/register"`,
		`"zord/cmd/http/handlers/auth/select_org"`,
		`"zord/pkg/registry"`,
		"func NewAuthRoute(reg *registry.Registry) *AuthRoute {",
		`g.POST("/"+prefix+"/auth"+"/login", r.loginHandler.Handle)`,
		`g.POST("/"+prefix+"/auth"+"/register", r.registerHandler.Handle)`,
		`g.POST("/"+prefix+"/auth"+"/select-org", r.selectOrgHandler.Handle)`,
	)
	// Campos da struct e atribuições do constructor passam por alinhamento
	// de colunas do gofmt (espaços variáveis entre `:` e o valor, conforme
	// o key mais longo), então usamos whitespace normalizado.
	mustContainNormalized(t, got,
		"loginHandler *login.LoginHandler",
		"registerHandler *register.RegisterHandler",
		"selectOrgHandler *select_org.SelectOrgHandler",
		"loginHandler: registry.Resolve[*login.LoginHandler](reg, login.RegistryKey)",
		"registerHandler: registry.Resolve[*register.RegisterHandler](reg, register.RegistryKey)",
		"selectOrgHandler: registry.Resolve[*select_org.SelectOrgHandler](reg, select_org.RegistryKey)",
	)
	// Cada chamada/import aparece exatamente uma vez.
	for _, line := range []string{
		`"zord/cmd/http/handlers/auth/login"`,
		`"zord/pkg/registry"`,
		`r.loginHandler.Handle`,
		`r.registerHandler.Handle`,
		`r.selectOrgHandler.Handle`,
	} {
		if c := strings.Count(got, line); c != 1 {
			t.Errorf("esperado 1 ocorrência de %q, got %d", line, c)
		}
	}
	// Cada atribuição de Resolve também aparece exatamente uma vez (após
	// normalizar whitespace pra absorver o alinhamento do gofmt).
	gotNorm := normalizeWhitespace(got)
	for _, line := range []string{
		"loginHandler: registry.Resolve[*login.LoginHandler](reg, login.RegistryKey)",
		"registerHandler: registry.Resolve[*register.RegisterHandler](reg, register.RegistryKey)",
		"selectOrgHandler: registry.Resolve[*select_org.SelectOrgHandler](reg, select_org.RegistryKey)",
	} {
		if c := strings.Count(gotNorm, normalizeWhitespace(line)); c != 1 {
			t.Errorf("esperado 1 ocorrência (normalizada) de %q, got %d", line, c)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "", got, parser.SkipObjectResolution); err != nil {
		t.Fatalf("arquivo final não parseia: %v\n%s", err, got)
	}
}

func TestRouteAdd_FailsIfRouteFileMissing(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")
	// rota não criada
	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
	if err == nil {
		t.Fatalf("RouteAdd: esperado erro pra route file ausente")
	}
}

func TestRouteAdd_FailsIfStructMissing(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	if _, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Auth"}); err != nil {
		t.Fatalf("RouteCreate: %v", err)
	}
	// quebra a estrutura: remove a struct AuthRoute
	stripped := "package routes\n\nimport \"github.com/labstack/echo/v4\"\n\nvar _ = echo.New\n"
	if err := os.WriteFile(filepath.Join(root, "cmd/http/routes/auth.go"), []byte(stripped), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")

	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
	if err == nil {
		t.Fatalf("RouteAdd: esperado erro pra struct ausente")
	}
	if !strings.Contains(err.Error(), "AuthRoute") {
		t.Errorf("erro %q não menciona AuthRoute", err.Error())
	}
}

func TestRouteAdd_FailsIfServiceMissing(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	if _, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Auth"}); err != nil {
		t.Fatalf("RouteCreate: %v", err)
	}
	// service não criado
	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
	if err == nil {
		t.Fatalf("RouteAdd: esperado erro pra service ausente")
	}
	if !strings.Contains(err.Error(), "service") {
		t.Errorf("erro %q não menciona service", err.Error())
	}
}

func TestRouteAdd_FailsIfServiceLacksRegistryKey(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")
	servicePath := filepath.Join(root, "internal/application/services/auth/login/service.go")
	stripped := "package login\nfunc NewService() {}\n"
	if err := os.WriteFile(servicePath, []byte(stripped), 0o600); err != nil {
		t.Fatalf("rewrite service: %v", err)
	}
	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
	if err == nil || !strings.Contains(err.Error(), "RegistryKey") {
		t.Fatalf("RouteAdd: esperado erro mencionando RegistryKey, got: %v", err)
	}
}

func TestRouteAdd_FailsIfHandlerMissing(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	if _, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Auth"}); err != nil {
		t.Fatalf("RouteCreate: %v", err)
	}
	seedService(t, root, "Auth", "Login")
	// handler não criado

	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
	if err == nil {
		t.Fatalf("RouteAdd: esperado erro pra handler ausente")
	}
	if !strings.Contains(err.Error(), "handler") {
		t.Errorf("erro %q não menciona handler", err.Error())
	}
}

func TestRouteAdd_FailsIfHandlerLacksHandleMethod(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")
	handlerPath := filepath.Join(root, "cmd/http/handlers/auth/login/handler.go")
	stripped := "package login\n\ntype LoginHandler struct{}\n"
	if err := os.WriteFile(handlerPath, []byte(stripped), 0o600); err != nil {
		t.Fatalf("rewrite handler: %v", err)
	}
	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
	if err == nil || !strings.Contains(err.Error(), "Handle") {
		t.Fatalf("RouteAdd: esperado erro mencionando Handle, got: %v", err)
	}
}

func TestRouteAdd_FailsIfRouteAlreadyAdded(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")

	if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST", Public: true}); err != nil {
		t.Fatalf("primeiro RouteAdd: %v", err)
	}
	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST", Public: true})
	if err == nil {
		t.Fatalf("segundo RouteAdd idêntico: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "já registrada") {
		t.Errorf("erro %q não menciona 'já registrada'", err.Error())
	}
}

func TestRouteAdd_FailsIfHandlerRegisteredInOppositeDeclare(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")

	if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST", Public: true}); err != nil {
		t.Fatalf("primeiro RouteAdd (public): %v", err)
	}
	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "GET"})
	if err == nil {
		t.Fatalf("RouteAdd (private): esperado erro pra handler já registrado, got nil")
	}
}

func TestRouteAdd_FailsIfMethodInvalid(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")
	cases := []string{"", "FETCH", "post body", "get,post"}
	for _, m := range cases {
		_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: m})
		if err == nil {
			t.Errorf("RouteAdd com method=%q: esperado erro, got nil", m)
		}
	}
}

func TestRouteAdd_NormalizesMethodCase(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Org", "GetOrg")

	if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Org", Service: "GetOrg", Method: "get"}); err != nil {
		t.Fatalf("RouteAdd: %v", err)
	}
	got := readFile(t, filepath.Join(root, "cmd/http/routes/org.go"))
	mustContain(t, got, `g.GET("/"+prefix+"/org"+"/get-org", r.getOrgHandler.Handle)`)
}

func TestRouteAdd_FailsIfDomainInvalid(t *testing.T) {
	_, err := RouteAdd(RouteAddOptions{Domain: "lowercase", Service: "Login", Method: "POST"})
	if err == nil {
		t.Fatalf("RouteAdd: esperado erro pra Domain inválido")
	}
}

func TestRouteAdd_FailsIfServiceInvalid(t *testing.T) {
	_, err := RouteAdd(RouteAddOptions{Domain: "Auth", Service: "lowercase", Method: "POST"})
	if err == nil {
		t.Fatalf("RouteAdd: esperado erro pra Service inválido")
	}
}

func TestRouteAdd_FailsIfRouteFileIsTypeAlias(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	if _, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Auth"}); err != nil {
		t.Fatalf("RouteCreate: %v", err)
	}
	// substitui AuthRoute por um type alias (não é struct).
	aliased := "package routes\n\nimport _ \"github.com/labstack/echo/v4\"\n\ntype AuthRoute = string\n"
	if err := os.WriteFile(filepath.Join(root, "cmd/http/routes/auth.go"), []byte(aliased), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	seedService(t, root, "Auth", "Login")
	seedHandler(t, root, "Auth", "Login")

	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
	if err == nil || !strings.Contains(err.Error(), "não é struct") {
		t.Fatalf("RouteAdd: esperado erro mencionando 'não é struct', got: %v", err)
	}
}

func TestRouteAdd_FailsIfCtorMissing(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")
	// remove o construtor mantendo struct + Declare*
	stripped := `package routes

import "github.com/labstack/echo/v4"

type AuthRoute struct {
}

func (r *AuthRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {
}

func (r *AuthRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {
}
`
	if err := os.WriteFile(filepath.Join(root, "cmd/http/routes/auth.go"), []byte(stripped), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
	if err == nil || !strings.Contains(err.Error(), "NewAuthRoute") {
		t.Fatalf("RouteAdd: esperado erro mencionando NewAuthRoute, got: %v", err)
	}
}

func TestRouteAdd_FailsIfDeclarePrivateMissing(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")
	stripped := `package routes

import (
	"zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

var _ *echo.Group

type AuthRoute struct {
}

func NewAuthRoute(reg *registry.Registry) *AuthRoute {
	return &AuthRoute{}
}

func (r *AuthRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {
}
`
	if err := os.WriteFile(filepath.Join(root, "cmd/http/routes/auth.go"), []byte(stripped), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
	if err == nil || !strings.Contains(err.Error(), "DeclarePrivateRoutes") {
		t.Fatalf("RouteAdd: esperado erro mencionando DeclarePrivateRoutes, got: %v", err)
	}
}

func TestRouteAdd_FailsIfDeclarePublicMissing(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")
	stripped := `package routes

import (
	"zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

var _ *echo.Group

type AuthRoute struct {
}

func NewAuthRoute(reg *registry.Registry) *AuthRoute {
	return &AuthRoute{}
}

func (r *AuthRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {
}
`
	if err := os.WriteFile(filepath.Join(root, "cmd/http/routes/auth.go"), []byte(stripped), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
	if err == nil || !strings.Contains(err.Error(), "DeclarePublicRoutes") {
		t.Fatalf("RouteAdd: esperado erro mencionando DeclarePublicRoutes, got: %v", err)
	}
}

func TestRouteAdd_FailsIfCtorSignatureNotCanonical(t *testing.T) {
	// Route hand-editada com parâmetros extras no constructor (middlewares,
	// deps cross-cutting, etc.) deve falhar — o scaffold só sabe gerar para
	// o shape canônico `New<Pascal>Route(reg *registry.Registry)`.
	cases := []struct {
		name string
		body string
	}{
		{
			name: "sem parâmetros",
			body: "func NewAuthRoute() *AuthRoute {\n\treturn &AuthRoute{}\n}",
		},
		{
			name: "parâmetro extra",
			body: "func NewAuthRoute(reg *registry.Registry, jwt string) *AuthRoute {\n\treturn &AuthRoute{}\n}",
		},
		{
			name: "tipo errado",
			body: "func NewAuthRoute(reg *string) *AuthRoute {\n\treturn &AuthRoute{}\n}",
		},
		{
			name: "primeiro parâmetro não chamado reg",
			body: "func NewAuthRoute(r *registry.Registry) *AuthRoute {\n\treturn &AuthRoute{}\n}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			seedAll(t, root, "Auth", "Login")
			hand := "package routes\n\nimport (\n\t\"zord/pkg/registry\"\n\n\t\"github.com/labstack/echo/v4\"\n)\n\nvar _ *echo.Group\n\ntype AuthRoute struct {\n}\n\n" + tc.body + "\n\nfunc (r *AuthRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {\n}\n\nfunc (r *AuthRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {\n}\n"
			if err := os.WriteFile(filepath.Join(root, "cmd/http/routes/auth.go"), []byte(hand), 0o600); err != nil {
				t.Fatalf("rewrite: %v", err)
			}
			_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
			if err == nil || !strings.Contains(err.Error(), "assinatura canônica") {
				t.Fatalf("RouteAdd: esperado erro mencionando 'assinatura canônica', got: %v", err)
			}
		})
	}
}

func TestRouteAdd_FailsIfRouteFileUnparseable(t *testing.T) {
	root := t.TempDir()
	seedAll(t, root, "Auth", "Login")
	// quebra sintaxe Go
	if err := os.WriteFile(filepath.Join(root, "cmd/http/routes/auth.go"), []byte("not go code"), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	_, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Auth", Service: "Login", Method: "POST"})
	if err == nil {
		t.Fatalf("RouteAdd: esperado erro pra arquivo unparseable")
	}
}

func TestRouteAdd_AllMethodsAccepted(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			root := t.TempDir()
			seedAll(t, root, "Org", "GetOrg")
			if _, err := RouteAdd(RouteAddOptions{Root: root, Domain: "Org", Service: "GetOrg", Method: method}); err != nil {
				t.Fatalf("RouteAdd %s: %v", method, err)
			}
			got := readFile(t, filepath.Join(root, "cmd/http/routes/org.go"))
			if !strings.Contains(got, "g."+method+"(") {
				t.Errorf("missing g.%s( in:\n%s", method, got)
			}
		})
	}
}

// --- seeding helpers (compartilhados com create_test.go) ---

// seedAll roda a cadeia completa domain → service → handler → route create
// para deixar o root pronto para um `route add`.
func seedAll(t *testing.T, root, dom, svc string) {
	t.Helper()
	seedDomain(t, root, dom)
	seedService(t, root, dom, svc)
	seedHandler(t, root, dom, svc)
	if _, err := RouteCreate(RouteCreateOptions{Root: root, Domain: dom}); err != nil {
		t.Fatalf("seed route %s: %v", dom, err)
	}
}

// mustContainNormalized checa substrings com whitespace colapsado a 1 espaço
// — útil para fixtures de campos de struct alinhados em colunas pelo gofmt
// (espaços variáveis entre nome e tipo conforme o campo mais longo).
func mustContainNormalized(t *testing.T, got string, wants ...string) {
	t.Helper()
	norm := normalizeWhitespace(got)
	for _, w := range wants {
		if !strings.Contains(norm, normalizeWhitespace(w)) {
			t.Errorf("missing %q (whitespace-normalized) in:\n%s", w, got)
		}
	}
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
