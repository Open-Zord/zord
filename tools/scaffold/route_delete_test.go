package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// declarableSemEntradas é canonicalDeclarable com o map vazio — pré-condição
// pra um `route delete` válido (o domain alvo nunca foi registrado ou já foi
// desregistrado).
const declarableSemEntradas = `package routes

import (
	"zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

type Declarable interface {
	DeclarePrivateRoutes(server *echo.Group, apiPrefix string)
	DeclarePublicRoutes(server *echo.Group, apiPrefix string)
}

func GetRoutes(reg *registry.Registry) map[string]Declarable {
	return map[string]Declarable{
		"health": NewHealthRoute(),
	}
}
`

// declarableComFoo simula declarable.go com a entrada "foo" ainda registrada
// — `route delete Foo` deve falhar nesse estado.
const declarableComFoo = `package routes

import (
	"zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

type Declarable interface {
	DeclarePrivateRoutes(server *echo.Group, apiPrefix string)
	DeclarePublicRoutes(server *echo.Group, apiPrefix string)
}

func GetRoutes(reg *registry.Registry) map[string]Declarable {
	return map[string]Declarable{
		"foo": NewFooRoute(reg),
	}
}
`

// fooRouteComHandler simula uma Route que já recebeu `route add` — struct
// populada com um campo handler. `route delete Foo` deve falhar listando
// o campo.
const fooRouteComHandler = `package routes

import (
	authhandler "zord/cmd/http/handlers/foo/auth"
	"zord/pkg/registry"

	"github.com/labstack/echo/v4"
)

type FooRoute struct {
	authHandler *authhandler.AuthHandler
}

func NewFooRoute(reg *registry.Registry) *FooRoute {
	return &FooRoute{
		authHandler: registry.Resolve[*authhandler.AuthHandler](reg, authhandler.RegistryKey),
	}
}

func (r *FooRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {
}

func (r *FooRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {
}
`

func TestRouteDelete_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, declarableSemEntradas)
	seedRouteFile(t, root, "Foo", canonicalRouteFile("Foo"))

	rel, err := RouteDelete(RouteDeleteOptions{Root: root, Domain: "Foo"})
	if err != nil {
		t.Fatalf("RouteDelete: %v", err)
	}
	if want := filepath.Join("cmd/http/routes/foo.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	abs := filepath.Join(root, rel)
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Errorf("arquivo %s ainda existe após delete: %v", rel, err)
	}
}

func TestRouteDelete_CompoundName(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, declarableSemEntradas)
	seedRouteFile(t, root, "UsageRecord", canonicalRouteFile("UsageRecord"))

	rel, err := RouteDelete(RouteDeleteOptions{Root: root, Domain: "UsageRecord"})
	if err != nil {
		t.Fatalf("RouteDelete: %v", err)
	}
	if want := filepath.Join("cmd/http/routes/usage_record.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
}

func TestRouteDelete_PassaSemDeclarable(t *testing.T) {
	// Em repos novos sem cmd/http/routes/declarable.go, o guarda passa por
	// vacuidade — o delete deve completar normalmente.
	root := t.TempDir()
	seedRouteFile(t, root, "Foo", canonicalRouteFile("Foo"))

	if _, err := RouteDelete(RouteDeleteOptions{Root: root, Domain: "Foo"}); err != nil {
		t.Fatalf("RouteDelete sem declarable.go: %v", err)
	}
}

func TestRouteDelete_FalhaSeArquivoAusente(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, declarableSemEntradas)

	_, err := RouteDelete(RouteDeleteOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro: arquivo ausente")
	}
	if !strings.Contains(err.Error(), "não existe") {
		t.Errorf("erro %q não menciona 'não existe'", err.Error())
	}
}

func TestRouteDelete_FalhaSeEntradaAindaRegistrada(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, declarableComFoo)
	seedRouteFile(t, root, "Foo", canonicalRouteFile("Foo"))

	_, err := RouteDelete(RouteDeleteOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro: entrada ainda em declarable.go")
	}
	if !strings.Contains(err.Error(), "\"foo\"") {
		t.Errorf("erro %q não cita a chave foo", err.Error())
	}
	// guarda não muta disco
	if _, err := os.Stat(filepath.Join(root, "cmd/http/routes/foo.go")); err != nil {
		t.Errorf("delete não devia ter removido o arquivo no erro: %v", err)
	}
}

func TestRouteDelete_FalhaSeStructTemCampos(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, declarableSemEntradas)
	seedRouteFile(t, root, "Foo", fooRouteComHandler)

	_, err := RouteDelete(RouteDeleteOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro: struct tem campos")
	}
	if !strings.Contains(err.Error(), "authHandler") {
		t.Errorf("erro %q não lista o campo authHandler", err.Error())
	}
	if _, err := os.Stat(filepath.Join(root, "cmd/http/routes/foo.go")); err != nil {
		t.Errorf("arquivo não devia ter sido removido: %v", err)
	}
}

func TestRouteDelete_InvalidDomain(t *testing.T) {
	_, err := RouteDelete(RouteDeleteOptions{Domain: "lowercase"})
	if err == nil {
		t.Fatalf("esperado erro pra Domain inválido")
	}
	_, err = RouteDelete(RouteDeleteOptions{Domain: ""})
	if err == nil {
		t.Fatalf("esperado erro pra Domain vazio")
	}
}

func TestRouteDelete_IdempotenciaInversa(t *testing.T) {
	// Primeiro delete passa; segundo deve falhar com "não existe".
	root := t.TempDir()
	seedDeclarable(t, root, declarableSemEntradas)
	seedRouteFile(t, root, "Foo", canonicalRouteFile("Foo"))

	if _, err := RouteDelete(RouteDeleteOptions{Root: root, Domain: "Foo"}); err != nil {
		t.Fatalf("primeiro RouteDelete: %v", err)
	}
	_, err := RouteDelete(RouteDeleteOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("segundo RouteDelete: esperado erro")
	}
}

func TestRouteDelete_DeclarableSemGetRoutes(t *testing.T) {
	// declarable.go existe mas não tem `GetRoutes` (estado degenerado).
	// O guarda do declarable trata como "sem registro a checar" e passa.
	root := t.TempDir()
	seedDeclarable(t, root, "package routes\n")
	seedRouteFile(t, root, "Foo", canonicalRouteFile("Foo"))

	if _, err := RouteDelete(RouteDeleteOptions{Root: root, Domain: "Foo"}); err != nil {
		t.Fatalf("RouteDelete sem GetRoutes: %v", err)
	}
}

func TestRouteDelete_FalhaSeRouteFileInvalido(t *testing.T) {
	// Arquivo da Route sem a struct esperada — guarda 2 falha apontando
	// a struct ausente.
	root := t.TempDir()
	seedDeclarable(t, root, declarableSemEntradas)
	seedRouteFile(t, root, "Foo", "package routes\n")

	_, err := RouteDelete(RouteDeleteOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro: struct ausente")
	}
	if !strings.Contains(err.Error(), "FooRoute") {
		t.Errorf("erro %q não menciona FooRoute", err.Error())
	}
}

func TestRouteDelete_FalhaSeRouteFileNaoParseia(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, declarableSemEntradas)
	seedRouteFile(t, root, "Foo", "package routes\nfunc broken( {\n")

	_, err := RouteDelete(RouteDeleteOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro: parse")
	}
}

func TestRouteDelete_FalhaSeDeclarableNaoParseia(t *testing.T) {
	root := t.TempDir()
	seedDeclarable(t, root, "package routes\nfunc broken( {\n")
	seedRouteFile(t, root, "Foo", canonicalRouteFile("Foo"))

	_, err := RouteDelete(RouteDeleteOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro: declarable não parseia")
	}
	if !strings.Contains(err.Error(), "declarable.go") {
		t.Errorf("erro %q não menciona declarable.go", err.Error())
	}
}
