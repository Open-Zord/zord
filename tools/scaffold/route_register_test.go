package scaffold

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouteRegister_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, canonicalDeclarable)
	seedRouteFile(t, root, "Namespace", canonicalRouteFile("Namespace"))

	rel, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Namespace"})
	if err != nil {
		t.Fatalf("RouteRegister: %v", err)
	}
	if want := filepath.Join("cmd/http/routes/declarable.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}

	got := readFile(t, filepath.Join(root, rel))
	// asserts tolerantes ao re-alinhamento de colunas do gofmt: "namespace" é
	// a maior chave do map pós-RouteRegister e força padding novo nas entradas
	// históricas. Verificamos chave e valor separadamente pra não acoplar ao
	// número exato de espaços.
	mustContain(t, got,
		// entrada nova (chave + chamada do constructor)
		`"namespace":`,
		`NewNamespaceRoute(reg),`,
		// entradas históricas preservadas (chave e valor cada um)
		`"health":`, `NewHealthRoute(),`,
		`"auth":`, `NewAuthRoute(authH, jwtValidator),`,
		`"org":`, `NewOrgRoute(orgH),`,
		// Resolves históricos preservados
		`authH := registry.Resolve[*authhandler.AuthHandler](reg, authhandler.RegistryKey)`,
		`orgH := registry.Resolve[*orghandler.OrgHandler](reg, orghandler.RegistryKey)`,
		// comentário preservado
		`// GetRoutes só resolve handlers prontos`,
	)
	// nova entrada vai pro fim do map (depois da última histórica "org")
	if idxOrg, idxNs := strings.Index(got, `"org"`), strings.Index(got, `"namespace"`); idxOrg == -1 || idxNs == -1 || idxOrg > idxNs {
		t.Errorf("entrada nova não ficou ao fim do map: org=%d, namespace=%d\n%s", idxOrg, idxNs, got)
	}
}

func TestRouteRegister_CompoundName(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, canonicalDeclarable)
	seedRouteFile(t, root, "UsageRecord", canonicalRouteFile("UsageRecord"))

	if _, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "UsageRecord"}); err != nil {
		t.Fatalf("RouteRegister: %v", err)
	}
	got := readFile(t, filepath.Join(root, "cmd/http/routes/declarable.go"))
	mustContain(t, got, `"usage_record": NewUsageRecordRoute(reg),`)
}

func TestRouteRegister_MultipleSequentialDomains(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, canonicalDeclarable)
	seedRouteFile(t, root, "Foo", canonicalRouteFile("Foo"))
	seedRouteFile(t, root, "Bar", canonicalRouteFile("Bar"))

	if _, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Foo"}); err != nil {
		t.Fatalf("RouteRegister Foo: %v", err)
	}
	if _, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Bar"}); err != nil {
		t.Fatalf("RouteRegister Bar: %v", err)
	}

	got := readFile(t, filepath.Join(root, "cmd/http/routes/declarable.go"))
	mustContain(t, got,
		`"foo":`, `NewFooRoute(reg),`,
		`"bar":`, `NewBarRoute(reg),`,
	)
	// ordem de inserção preservada: Foo (inserido primeiro) aparece antes de Bar
	if iFoo, iBar := strings.Index(got, `"foo"`), strings.Index(got, `"bar"`); iFoo == -1 || iBar == -1 || iFoo > iBar {
		t.Errorf("ordem de inserção quebrada: foo=%d, bar=%d", iFoo, iBar)
	}
}

func TestRouteRegister_InvalidDomainNames(t *testing.T) {
	for _, bad := range []string{"", "lowercase", "1Number", "kebab-case"} {
		bad := bad
		t.Run(bad, func(t *testing.T) {
			if _, err := RouteRegister(RouteRegisterOptions{Domain: bad}); err == nil {
				t.Fatalf("esperado erro pra Domain %q", bad)
			}
		})
	}
}

func TestRouteRegister_FailsIfDeclarableMissing(t *testing.T) {
	root := t.TempDir()
	seedRouteFile(t, root, "Foo", canonicalRouteFile("Foo"))

	_, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro pra declarable.go ausente")
	}
	if !strings.Contains(err.Error(), "declarable.go") {
		t.Errorf("erro %q não menciona declarable.go", err.Error())
	}
}

func TestRouteRegister_FailsIfGetRoutesMissing(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, "package routes\n")
	seedRouteFile(t, root, "Foo", canonicalRouteFile("Foo"))

	_, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro pra GetRoutes ausente")
	}
	if !strings.Contains(err.Error(), "GetRoutes") {
		t.Errorf("erro %q não menciona GetRoutes", err.Error())
	}
}

func TestRouteRegister_FailsIfReturnNotMap(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, `package routes

type Declarable interface{}

func GetRoutes() map[string]Declarable {
	return nil
}
`)
	seedRouteFile(t, root, "Foo", canonicalRouteFile("Foo"))

	_, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro pra return sem map literal")
	}
}

func TestRouteRegister_FailsIfRouteMissing(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, canonicalDeclarable)
	// nada de seedRouteFile

	_, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro pra Route ausente")
	}
}

func TestRouteRegister_FailsIfRouteStructMissing(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, canonicalDeclarable)
	seedRouteFile(t, root, "Foo", "package routes\n\nfunc NewFooRoute(reg *registry.Registry) *FooRoute { return nil }\n")

	_, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro pra struct ausente")
	}
	if !strings.Contains(err.Error(), "FooRoute") {
		t.Errorf("erro %q não menciona FooRoute", err.Error())
	}
}

func TestRouteRegister_FailsIfCtorMissing(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, canonicalDeclarable)
	seedRouteFile(t, root, "Foo", "package routes\n\ntype FooRoute struct{}\n")

	_, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro pra constructor ausente")
	}
	if !strings.Contains(err.Error(), "NewFooRoute") {
		t.Errorf("erro %q não menciona NewFooRoute", err.Error())
	}
}

func TestRouteRegister_FailsIfCtorHasExtraParam(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, canonicalDeclarable)
	seedRouteFile(t, root, "Foo", `package routes

import "zord/pkg/registry"

type FooRoute struct{}

func NewFooRoute(reg *registry.Registry, extra string) *FooRoute {
	return &FooRoute{}
}
`)

	_, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro pra ctor com parâmetro extra")
	}
	if !strings.Contains(err.Error(), "assinatura canônica") {
		t.Errorf("erro %q não menciona assinatura canônica", err.Error())
	}
}

func TestRouteRegister_FailsIfCtorWrongType(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, canonicalDeclarable)
	seedRouteFile(t, root, "Foo", `package routes

type FooRoute struct{}

func NewFooRoute(reg string) *FooRoute {
	return &FooRoute{}
}
`)

	_, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro pra ctor com tipo errado de reg")
	}
}

func TestRouteRegister_FailsIfEntryAlreadyExists(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, canonicalDeclarable)
	seedRouteFile(t, root, "Foo", canonicalRouteFile("Foo"))

	if _, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Foo"}); err != nil {
		t.Fatalf("primeiro RouteRegister: %v", err)
	}
	_, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro na segunda execução (idempotência)")
	}
	if !strings.Contains(err.Error(), `"foo"`) {
		t.Errorf("erro %q não cita a chave duplicada", err.Error())
	}
}

func TestRouteRegister_DeclarableIntactOnFailure(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
		opts  RouteRegisterOptions
	}{
		{
			name: "domain inválido",
			setup: func(t *testing.T, root string) {
				seedDeclarable(t, root, canonicalDeclarable)
			},
			opts: RouteRegisterOptions{Domain: "lowercase"},
		},
		{
			name: "route ausente",
			setup: func(t *testing.T, root string) {
				seedDeclarable(t, root, canonicalDeclarable)
			},
			opts: RouteRegisterOptions{Domain: "Foo"},
		},
		{
			name: "ctor com parâmetro extra",
			setup: func(t *testing.T, root string) {
				seedDeclarable(t, root, canonicalDeclarable)
				seedRouteFile(t, root, "Foo", `package routes

import "zord/pkg/registry"

type FooRoute struct{}

func NewFooRoute(reg *registry.Registry, extra string) *FooRoute { return &FooRoute{} }
`)
			},
			opts: RouteRegisterOptions{Domain: "Foo"},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			c.setup(t, root)
			c.opts.Root = root

			before, err := os.ReadFile(filepath.Join(root, "cmd/http/routes/declarable.go"))
			if err != nil {
				t.Fatalf("read before: %v", err)
			}
			if _, err := RouteRegister(c.opts); err == nil {
				t.Fatalf("esperado erro")
			}
			after, err := os.ReadFile(filepath.Join(root, "cmd/http/routes/declarable.go"))
			if err != nil {
				t.Fatalf("read after: %v", err)
			}
			if string(before) != string(after) {
				t.Errorf("declarable.go foi mutado em falha\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

func TestRouteRegister_GeneratedFileIsValidGo(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, canonicalDeclarable)
	seedRouteFile(t, root, "Namespace", canonicalRouteFile("Namespace"))

	rel, err := RouteRegister(RouteRegisterOptions{Root: root, Domain: "Namespace"})
	if err != nil {
		t.Fatalf("RouteRegister: %v", err)
	}
	src, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution); err != nil {
		t.Fatalf("declarable.go pós-RouteRegister não parsea: %v\n%s", err, src)
	}
}

// --- fixtures e seeding helpers ---

// canonicalDeclarable é o conteúdo de referência de cmd/http/routes/declarable.go
// pré-RouteRegister — espelha o arquivo real em produção (cmd/http/routes/declarable.go
// em origin/main). O `RouteRegister` só insere uma entrada no map; tudo o mais
// (imports, Resolves, comentário, entradas históricas) permanece intocado.
const canonicalDeclarable = `package routes

import (
	authhandler "zord/cmd/http/handlers/auth"
	orghandler "zord/cmd/http/handlers/org"
	pkgjwt "zord/pkg/jwt"
	"zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

type Declarable interface {
	DeclarePrivateRoutes(server *echo.Group, apiPrefix string)
	DeclarePublicRoutes(server *echo.Group, apiPrefix string)
}

// GetRoutes só resolve handlers prontos do registry — nenhuma construção de
// service, repositório ou adapter mora aqui. Wiring fica em bootstrap/.
func GetRoutes(reg *registry.Registry) map[string]Declarable {
	authH := registry.Resolve[*authhandler.AuthHandler](reg, authhandler.RegistryKey)
	orgH := registry.Resolve[*orghandler.OrgHandler](reg, orghandler.RegistryKey)
	jwtValidator := registry.Resolve[*pkgjwt.LocalValidator](reg, pkgjwt.RegistryKey)

	return map[string]Declarable{
		"health": NewHealthRoute(),
		"auth":   NewAuthRoute(authH, jwtValidator),
		"org":    NewOrgRoute(orgH),
	}
}
`

// canonicalRouteFile devolve o conteúdo da Route gerada pela NAVE-74 — struct
// vazia, ctor com `reg *registry.Registry`, Declare* sem corpo. É o mesmo
// shape que `route create` emite (validado pelos testes da NAVE-74).
func canonicalRouteFile(domain string) string {
	return `package routes

import (
	"zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

type ` + domain + `Route struct {
}

func New` + domain + `Route(reg *registry.Registry) *` + domain + `Route {
	return &` + domain + `Route{}
}

func (r *` + domain + `Route) DeclarePrivateRoutes(g *echo.Group, prefix string) {
}

func (r *` + domain + `Route) DeclarePublicRoutes(g *echo.Group, prefix string) {
}
`
}

func seedDeclarable(t *testing.T, root, content string) {
	t.Helper()
	abs := filepath.Join(root, "cmd/http/routes/declarable.go")
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		t.Fatalf("seed declarable: %v", err)
	}
}

func seedRouteFile(t *testing.T, root, domain, content string) {
	t.Helper()
	snake := snakeOf(domain)
	abs := filepath.Join(root, "cmd/http/routes", snake+".go")
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		t.Fatalf("seed route %s: %v", domain, err)
	}
}

// snakeOf é um helper local mínimo só pra evitar dependência cruzada com o
// pacote `name` nos testes. Cobre os casos usados aqui: "Foo" → "foo",
// "UsageRecord" → "usage_record".
func snakeOf(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
