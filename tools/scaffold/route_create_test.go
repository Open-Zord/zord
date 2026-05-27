package scaffold

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouteCreate_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")

	rel, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Auth"})
	if err != nil {
		t.Fatalf("RouteCreate: %v", err)
	}
	if want := filepath.Join("cmd/http/routes/auth.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		"package routes",
		`"zord/pkg/registry"`,
		`"github.com/labstack/echo/v4"`,
		"type AuthRoute struct {",
		"func NewAuthRoute(reg *registry.Registry) *AuthRoute {",
		"return &AuthRoute{}",
		"func (r *AuthRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {",
		"func (r *AuthRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {",
	)
}

func TestRouteCreate_CompoundName(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "UsageRecord")

	rel, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "UsageRecord"})
	if err != nil {
		t.Fatalf("RouteCreate: %v", err)
	}
	if want := filepath.Join("cmd/http/routes/usage_record.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		`"zord/pkg/registry"`,
		"type UsageRecordRoute struct {",
		"func NewUsageRecordRoute(reg *registry.Registry) *UsageRecordRoute {",
		"return &UsageRecordRoute{}",
		"func (r *UsageRecordRoute) DeclarePrivateRoutes(g *echo.Group, prefix string) {",
		"func (r *UsageRecordRoute) DeclarePublicRoutes(g *echo.Group, prefix string) {",
	)
}

func TestRouteCreate_FailsIfDomainMissing(t *testing.T) {
	root := t.TempDir()
	if _, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Missing"}); err == nil {
		t.Fatalf("RouteCreate: esperado erro pra domínio inexistente")
	}
}

func TestRouteCreate_FailsIfDomainStructMissing(t *testing.T) {
	root := t.TempDir()
	rel := seedDomain(t, root, "Foo")
	// substitui o conteúdo do arquivo do domínio por apenas a declaração de pacote
	if err := os.WriteFile(filepath.Join(root, rel), []byte("package foo\n"), 0o600); err != nil {
		t.Fatalf("rewrite domain: %v", err)
	}
	_, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("RouteCreate: esperado erro pra struct inexistente")
	}
	if !strings.Contains(err.Error(), "Foo") {
		t.Errorf("erro %q não menciona Foo", err.Error())
	}
}

func TestRouteCreate_FailsIfRouteAlreadyExists(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Foo")
	if _, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Foo"}); err != nil {
		t.Fatalf("primeiro RouteCreate: %v", err)
	}
	_, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("segundo RouteCreate: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "já existe") {
		t.Errorf("erro %q não menciona 'já existe'", err.Error())
	}
}

func TestRouteCreate_InvalidDomainName(t *testing.T) {
	_, err := RouteCreate(RouteCreateOptions{Domain: "lowercase"})
	if err == nil {
		t.Fatalf("RouteCreate: esperado erro pra Domain inválido")
	}
}

func TestRouteCreate_GeneratedFileIsValidGo(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	rel, err := RouteCreate(RouteCreateOptions{Root: root, Domain: "Auth"})
	if err != nil {
		t.Fatalf("RouteCreate: %v", err)
	}
	src, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution); err != nil {
		t.Fatalf("arquivo gerado não compila no parser: %v\n%s", err, src)
	}
}

// --- seeding helpers ---
