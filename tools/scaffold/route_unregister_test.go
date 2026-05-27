package scaffold

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouteUnregister_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, declarableWithNamespace)

	rel, err := RouteUnregister(RouteUnregisterOptions{Root: root, Domain: "Namespace"})
	if err != nil {
		t.Fatalf("RouteUnregister: %v", err)
	}
	if want := filepath.Join("cmd/http/routes/declarable.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}

	got := readFile(t, filepath.Join(root, rel))
	if strings.Contains(got, `"namespace"`) {
		t.Errorf("chave \"namespace\" ainda presente após unregister:\n%s", got)
	}
	if strings.Contains(got, `NewNamespaceRoute`) {
		t.Errorf("chamada NewNamespaceRoute ainda presente após unregister:\n%s", got)
	}
	// entradas históricas preservadas
	mustContain(t, got,
		`"health":`, `NewHealthRoute(),`,
		`"auth":`, `NewAuthRoute(authH, jwtValidator),`,
		`"org":`, `NewOrgRoute(orgH),`,
		`authH := registry.Resolve[*authhandler.AuthHandler](reg, authhandler.RegistryKey)`,
		`orgH := registry.Resolve[*orghandler.OrgHandler](reg, orghandler.RegistryKey)`,
		`// GetRoutes só resolve handlers prontos`,
	)
}

func TestRouteUnregister_CompoundName(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, declarableWithUsageRecord)

	if _, err := RouteUnregister(RouteUnregisterOptions{Root: root, Domain: "UsageRecord"}); err != nil {
		t.Fatalf("RouteUnregister: %v", err)
	}
	got := readFile(t, filepath.Join(root, "cmd/http/routes/declarable.go"))
	if strings.Contains(got, `"usage_record"`) {
		t.Errorf("chave \"usage_record\" ainda presente:\n%s", got)
	}
}

func TestRouteUnregister_FailsIfKeyMissing(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, declarableWithNamespace)

	// primeira execução: ok
	if _, err := RouteUnregister(RouteUnregisterOptions{Root: root, Domain: "Namespace"}); err != nil {
		t.Fatalf("primeiro unregister: %v", err)
	}
	// segunda execução: deve falhar
	_, err := RouteUnregister(RouteUnregisterOptions{Root: root, Domain: "Namespace"})
	if err == nil {
		t.Fatalf("esperado erro na segunda execução (idempotência negativa)")
	}
	if !strings.Contains(err.Error(), `"namespace"`) {
		t.Errorf("erro %q não cita a chave ausente", err.Error())
	}
	if !strings.Contains(err.Error(), "GetRoutes") {
		t.Errorf("erro %q não cita GetRoutes", err.Error())
	}
}

func TestRouteUnregister_FailsIfKeyNeverPresent(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, canonicalDeclarable) // sem "namespace"

	_, err := RouteUnregister(RouteUnregisterOptions{Root: root, Domain: "Namespace"})
	if err == nil {
		t.Fatalf("esperado erro pra chave nunca presente")
	}
	if !strings.Contains(err.Error(), `"namespace"`) {
		t.Errorf("erro %q não cita a chave", err.Error())
	}
}

func TestRouteUnregister_InvalidDomainNames(t *testing.T) {
	for _, bad := range []string{"", "lowercase", "1Number", "kebab-case"} {
		bad := bad
		t.Run(bad, func(t *testing.T) {
			if _, err := RouteUnregister(RouteUnregisterOptions{Domain: bad}); err == nil {
				t.Fatalf("esperado erro pra Domain %q", bad)
			}
		})
	}
}

func TestRouteUnregister_FailsIfDeclarableMissing(t *testing.T) {
	root := t.TempDir()
	// nada de seedDeclarable

	_, err := RouteUnregister(RouteUnregisterOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro pra declarable.go ausente")
	}
	if !strings.Contains(err.Error(), "declarable.go") {
		t.Errorf("erro %q não menciona declarable.go", err.Error())
	}
}

func TestRouteUnregister_FailsIfGetRoutesMissing(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, "package routes\n")

	_, err := RouteUnregister(RouteUnregisterOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro pra GetRoutes ausente")
	}
	if !strings.Contains(err.Error(), "GetRoutes") {
		t.Errorf("erro %q não menciona GetRoutes", err.Error())
	}
}

func TestRouteUnregister_FailsIfReturnNotMap(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, `package routes

type Declarable interface{}

func GetRoutes() map[string]Declarable {
	return nil
}
`)

	_, err := RouteUnregister(RouteUnregisterOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro pra return sem map literal")
	}
}

func TestRouteUnregister_DeclarableIntactOnFailure(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
		opts  RouteUnregisterOptions
	}{
		{
			name: "domain inválido",
			setup: func(t *testing.T, root string) {
				seedDeclarable(t, root, declarableWithNamespace)
			},
			opts: RouteUnregisterOptions{Domain: "lowercase"},
		},
		{
			name: "chave ausente",
			setup: func(t *testing.T, root string) {
				seedDeclarable(t, root, canonicalDeclarable)
			},
			opts: RouteUnregisterOptions{Domain: "Foo"},
		},
		{
			name: "GetRoutes ausente",
			setup: func(t *testing.T, root string) {
				seedDeclarable(t, root, "package routes\n")
			},
			opts: RouteUnregisterOptions{Domain: "Foo"},
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
			if _, err := RouteUnregister(c.opts); err == nil {
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

func TestRouteUnregister_GeneratedFileIsValidGo(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, declarableWithNamespace)

	rel, err := RouteUnregister(RouteUnregisterOptions{Root: root, Domain: "Namespace"})
	if err != nil {
		t.Fatalf("RouteUnregister: %v", err)
	}
	src, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution); err != nil {
		t.Fatalf("declarable.go pós-RouteUnregister não parsea: %v\n%s", err, src)
	}
}

func TestRouteUnregister_PreservesLegacyEntries(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, declarableFull)

	// remove usage_record do meio das entradas NAVE-65
	if _, err := RouteUnregister(RouteUnregisterOptions{Root: root, Domain: "UsageRecord"}); err != nil {
		t.Fatalf("RouteUnregister UsageRecord: %v", err)
	}

	got := readFile(t, filepath.Join(root, "cmd/http/routes/declarable.go"))

	// chave removida sumiu
	if strings.Contains(got, `"usage_record"`) {
		t.Errorf("chave \"usage_record\" ainda presente:\n%s", got)
	}
	if strings.Contains(got, "NewUsageRecordRoute") {
		t.Errorf("NewUsageRecordRoute ainda presente:\n%s", got)
	}

	// 5 entradas restantes preservadas
	mustContain(t, got,
		`"health":`, `NewHealthRoute(),`,
		`"auth":`, `NewAuthRoute(authH, jwtValidator),`,
		`"org":`, `NewOrgRoute(orgH),`,
		`"namespace":`, `NewNamespaceRoute(reg),`,
		`"session":`, `NewSessionRoute(reg),`,
	)

	// ordem relativa preservada das chaves NAVE-65: namespace antes de session
	idxNs := strings.Index(got, `"namespace"`)
	idxSess := strings.Index(got, `"session"`)
	if idxNs == -1 || idxSess == -1 || idxNs > idxSess {
		t.Errorf("ordem relativa quebrada: namespace=%d, session=%d", idxNs, idxSess)
	}

	// chaves legadas continuam antes das NAVE-65
	idxOrg := strings.Index(got, `"org"`)
	if idxOrg == -1 || idxOrg > idxNs {
		t.Errorf("ordem legado-vs-NAVE-65 quebrada: org=%d, namespace=%d", idxOrg, idxNs)
	}
}

// --- fixtures locais (helpers de seeding/readFile/mustContain vêm dos
// outros _test.go do pacote) ---

// declarableWithNamespace é o canonicalDeclarable com a entrada "namespace"
// já inserida (estado pós-RouteRegister para Namespace).
const declarableWithNamespace = `package routes

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
		"health":    NewHealthRoute(),
		"auth":      NewAuthRoute(authH, jwtValidator),
		"org":       NewOrgRoute(orgH),
		"namespace": NewNamespaceRoute(reg),
	}
}
`

// declarableWithUsageRecord cobre o caso de chave composta (snake_case
// derivada de PascalCase composto).
const declarableWithUsageRecord = `package routes

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

func GetRoutes(reg *registry.Registry) map[string]Declarable {
	authH := registry.Resolve[*authhandler.AuthHandler](reg, authhandler.RegistryKey)
	orgH := registry.Resolve[*orghandler.OrgHandler](reg, orghandler.RegistryKey)
	jwtValidator := registry.Resolve[*pkgjwt.LocalValidator](reg, pkgjwt.RegistryKey)

	return map[string]Declarable{
		"health":       NewHealthRoute(),
		"auth":         NewAuthRoute(authH, jwtValidator),
		"org":          NewOrgRoute(orgH),
		"usage_record": NewUsageRecordRoute(reg),
	}
}
`

// declarableFull tem entradas legadas + 3 chaves NAVE-65 (namespace,
// session, usage_record). Cobre o cenário do CKP4: remover uma chave do
// meio e validar que as outras 5 permanecem intactas.
const declarableFull = `package routes

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

func GetRoutes(reg *registry.Registry) map[string]Declarable {
	authH := registry.Resolve[*authhandler.AuthHandler](reg, authhandler.RegistryKey)
	orgH := registry.Resolve[*orghandler.OrgHandler](reg, orghandler.RegistryKey)
	jwtValidator := registry.Resolve[*pkgjwt.LocalValidator](reg, pkgjwt.RegistryKey)

	return map[string]Declarable{
		"health":       NewHealthRoute(),
		"auth":         NewAuthRoute(authH, jwtValidator),
		"org":          NewOrgRoute(orgH),
		"namespace":    NewNamespaceRoute(reg),
		"session":      NewSessionRoute(reg),
		"usage_record": NewUsageRecordRoute(reg),
	}
}
`
