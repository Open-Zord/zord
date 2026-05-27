package scaffold

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlerCreate_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")

	rel, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err != nil {
		t.Fatalf("HandlerCreate: %v", err)
	}
	if want := filepath.Join("cmd/http/handlers/auth/login/handler.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		"// Package login expõe o handler HTTP do use case Login.",
		"package login",
		`"net/http"`,
		`"zord/cmd/http/httperr"`,
		`"zord/internal/application/services/auth/login"`,
		`"zord/pkg/registry"`,
		`"github.com/labstack/echo/v4"`,
		"// RegistryKey identifica o *LoginHandler no pkg/registry.",
		`const RegistryKey = "loginHandler"`,
		"// LoginHandler atende o use case Login. Mantém as deps já resolvidas pelo New.",
		"type LoginHandler struct {",
		"svc *login.Service",
		"// NewLoginHandler resolve as dependências do handler no registry da aplicação. Falha de resolução quebra Setup() (proposital — falha rápida).",
		"func NewLoginHandler(reg *registry.Registry) *LoginHandler",
		"svc := registry.Resolve[*login.Service](reg, login.RegistryKey)",
		"return &LoginHandler{svc: svc}",
		"// Handle executa o use case Login.",
		"func (h *LoginHandler) Handle(c echo.Context) error",
		"var data login.Data",
		"if err := c.Bind(&data); err != nil",
		"return httperr.RespondBadRequest(c, err.Error())",
		"req := login.NewRequest(&data)",
		"if err := h.svc.Execute(c.Request().Context(), req); err != nil",
		"return httperr.Respond(c, err)",
		"out, _ := h.svc.GetResponse()",
		"return c.JSON(http.StatusOK, out)",
	)
	mustNotContain(t, got,
		"return &LoginHandler{reg: reg}",
		"h.reg",
		"login.Input",
		"login.Output",
		"RespondServiceError",
		"*services.Error",
	)
}

func TestHandlerCreate_WithValidator(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	if _, err := RequestValidatorSet(RequestValidatorOptions{Root: root, Domain: "Auth", Verb: "Login"}); err != nil {
		t.Fatalf("RequestValidatorSet: %v", err)
	}

	rel, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err != nil {
		t.Fatalf("HandlerCreate: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		`"zord/internal/application/services"`,
		`"zord/pkg/validator"`,
		"validator services.Validator",
		"valSvc := registry.Resolve[services.Validator](reg, validator.RegistryKey)",
		"return &LoginHandler{svc: svc, validator: valSvc}",
		"req := login.NewRequest(&data, h.validator)",
	)
	if err := parseGoSrc([]byte(got)); err != nil {
		t.Fatalf("handler gerado inválido: %v\n%s", err, got)
	}
}

func TestHandlerCreate_CompoundNames(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "UsageRecord")
	seedService(t, root, "UsageRecord", "Export")

	rel, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: "UsageRecord", Service: "Export"})
	if err != nil {
		t.Fatalf("HandlerCreate: %v", err)
	}
	if want := filepath.Join("cmd/http/handlers/usage_record/export/handler.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		"package export",
		`"zord/internal/application/services/usage_record/export"`,
		`const RegistryKey = "exportHandler"`,
		"type ExportHandler struct {",
		"svc *export.Service",
		"func NewExportHandler(reg *registry.Registry) *ExportHandler",
		"svc := registry.Resolve[*export.Service](reg, export.RegistryKey)",
		"return &ExportHandler{svc: svc}",
		"func (h *ExportHandler) Handle(c echo.Context) error",
		"var data export.Data",
		"req := export.NewRequest(&data)",
		"if err := h.svc.Execute(c.Request().Context(), req); err != nil",
		"out, _ := h.svc.GetResponse()",
	)
}

func TestHandlerCreate_FailsIfHandlerAlreadyExists(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Foo")
	seedService(t, root, "Foo", "Bar")

	if _, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: "Foo", Service: "Bar"}); err != nil {
		t.Fatalf("primeiro HandlerCreate: %v", err)
	}
	_, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: "Foo", Service: "Bar"})
	if err == nil {
		t.Fatalf("segundo HandlerCreate: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "já existe") {
		t.Errorf("erro %q não menciona 'já existe'", err.Error())
	}
}

func TestHandlerCreate_FailsIfDomainFileMissing(t *testing.T) {
	root := t.TempDir()
	_, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: "Missing", Service: "Login"})
	if err == nil {
		t.Fatalf("HandlerCreate: esperado erro pra domínio inexistente")
	}
}

func TestHandlerCreate_FailsIfDomainStructMissing(t *testing.T) {
	root := t.TempDir()
	rel := seedDomain(t, root, "Foo")
	seedService(t, root, "Foo", "Bar")
	if err := os.WriteFile(filepath.Join(root, rel), []byte("package foo\n"), 0o600); err != nil {
		t.Fatalf("rewrite domain: %v", err)
	}
	_, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: "Foo", Service: "Bar"})
	if err == nil {
		t.Fatalf("HandlerCreate: esperado erro pra struct inexistente")
	}
	if !strings.Contains(err.Error(), "Foo") {
		t.Errorf("erro %q não menciona Foo", err.Error())
	}
}

func TestHandlerCreate_FailsIfServiceMissing(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")

	_, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("HandlerCreate: esperado erro pra service inexistente")
	}
	if !strings.Contains(err.Error(), "service") {
		t.Errorf("erro %q não menciona service", err.Error())
	}
}

func TestHandlerCreate_FailsIfServiceLacksRegistryKey(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	servicePath := filepath.Join(root, "internal/application/services/auth/login/service.go")
	stripped := "package login\nfunc NewService() {}\n"
	if err := os.WriteFile(servicePath, []byte(stripped), 0o600); err != nil {
		t.Fatalf("rewrite service: %v", err)
	}

	_, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("HandlerCreate: esperado erro pra RegistryKey ausente")
	}
	if !strings.Contains(err.Error(), "RegistryKey") {
		t.Errorf("erro %q não menciona RegistryKey", err.Error())
	}
}

func TestHandlerCreate_FailsIfServiceLacksNewService(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	servicePath := filepath.Join(root, "internal/application/services/auth/login/service.go")
	stripped := "package login\nconst RegistryKey = \"loginService\"\n"
	if err := os.WriteFile(servicePath, []byte(stripped), 0o600); err != nil {
		t.Fatalf("rewrite service: %v", err)
	}

	_, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err == nil {
		t.Fatalf("HandlerCreate: esperado erro pra NewService ausente")
	}
	if !strings.Contains(err.Error(), "NewService") {
		t.Errorf("erro %q não menciona NewService", err.Error())
	}
}

func TestHandlerCreate_InvalidDomainName(t *testing.T) {
	_, err := HandlerCreate(HandlerCreateOptions{Domain: "lowercase", Service: "Login"})
	if err == nil {
		t.Fatalf("HandlerCreate: esperado erro pra Domain inválido")
	}
}

func TestHandlerCreate_InvalidServiceName(t *testing.T) {
	_, err := HandlerCreate(HandlerCreateOptions{Domain: "Auth", Service: "lowercase"})
	if err == nil {
		t.Fatalf("HandlerCreate: esperado erro pra Service inválido")
	}
}

func TestHandlerCreate_FailsLeavesHandlerDirUntouched(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	if _, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: "Auth", Service: "Login"}); err == nil {
		t.Fatalf("HandlerCreate: esperado erro")
	}
	if _, err := os.Stat(filepath.Join(root, "cmd/http/handlers/auth/login")); err == nil {
		t.Errorf("pasta do handler foi criada apesar do erro")
	} else if !os.IsNotExist(err) {
		t.Errorf("stat: %v", err)
	}
}

func TestHandlerCreate_GeneratedFileIsValidGo(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Auth")
	seedService(t, root, "Auth", "Login")
	rel, err := HandlerCreate(HandlerCreateOptions{Root: root, Domain: "Auth", Service: "Login"})
	if err != nil {
		t.Fatalf("HandlerCreate: %v", err)
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

func seedDomain(t *testing.T, root, typeName string) string {
	t.Helper()
	rel, err := DomainCreate(typeName, DomainCreateOptions{Root: root})
	if err != nil {
		t.Fatalf("seed domain %s: %v", typeName, err)
	}
	return rel
}

func seedService(t *testing.T, root, dom, verb string) {
	t.Helper()
	if _, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: dom, Verb: verb}); err != nil {
		t.Fatalf("seed service %s/%s: %v", dom, verb, err)
	}
}

func mustContain(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}

func mustNotContain(t *testing.T, got string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(got, u) {
			t.Errorf("unexpected %q in:\n%s", u, got)
		}
	}
}
