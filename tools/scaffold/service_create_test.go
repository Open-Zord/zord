package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceCreate_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Project")

	paths, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "Project", Verb: "Login"})
	if err != nil {
		t.Fatalf("ServiceCreate: %v", err)
	}
	wantPaths := []string{
		filepath.Join("internal/application/services/project/login/request.go"),
		filepath.Join("internal/application/services/project/login/service.go"),
		filepath.Join("internal/application/services/project/login/response.go"),
	}
	if len(paths) != len(wantPaths) {
		t.Fatalf("paths: got %v, want %v", paths, wantPaths)
	}
	for i, want := range wantPaths {
		if paths[i] != want {
			t.Errorf("paths[%d]: got %q, want %q", i, paths[i], want)
		}
	}

	gotReq := readFile(t, filepath.Join(root, paths[0]))
	mustContain(t, gotReq,
		"package login",
		"// Data agrega os campos de entrada validáveis do use case Login.",
		"type Data struct{",
		"// Request encapsula Data para o Execute do Service.",
		"type Request struct {",
		"Data *Data",
		"// NewRequest constrói o Request.",
		"func NewRequest(data *Data) *Request",
		"return &Request{Data: data}",
		"// Validate valida Data. Sem validator configurado, retorna nil.",
		"func (r *Request) Validate() error",
		"return nil",
	)
	mustNotContain(t, gotReq, "// Package login implementa")

	gotSvc := readFile(t, filepath.Join(root, paths[1]))
	mustContain(t, gotSvc,
		"// Package login implementa o use case Login.",
		"package login",
		`"context"`,
		`"zord/internal/application/services"`,
		"// RegistryKey identifica o *Service no pkg/registry.",
		`const RegistryKey = "loginService"`,
		"// Service executa o use case Login.",
		"type Service struct {",
		"services.BaseService",
		"response *Response",
		"// NewService constrói o Service com suas dependências.",
		"func NewService(logger services.Logger, idCreator services.IdCreator) *Service",
		"BaseService: services.BaseService{Logger: logger, Ulid: idCreator}",
		"// Execute roda o use case Login.",
		"func (s *Service) Execute(_ context.Context, request *Request) error",
		"if err := request.Validate(); err != nil",
		"return services.NewInvalid(err.Error())",
		"s.response = &Response{}",
		"return nil",
		"// GetResponse devolve a resposta produzida pelo Execute.",
		"func (s *Service) GetResponse() (*Response, error)",
		"return s.response, nil",
	)
	mustNotContain(t, gotSvc, "*services.Error", "s.BadRequest", "s.Error")

	gotResp := readFile(t, filepath.Join(root, paths[2]))
	mustContain(t, gotResp,
		"package login",
		"// Response agrega a saída do use case Login.",
		"type Response struct{",
	)
	mustNotContain(t, gotResp, "// Package login implementa")
}

func TestServiceCreate_CompoundVerb(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "User")

	paths, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "User", Verb: "SelectOrg"})
	if err != nil {
		t.Fatalf("ServiceCreate: %v", err)
	}
	if want := filepath.Join("internal/application/services/user/select_org/service.go"); paths[1] != want {
		t.Errorf("service path: got %q, want %q", paths[1], want)
	}

	gotSvc := readFile(t, filepath.Join(root, paths[1]))
	mustContain(t, gotSvc,
		"// Package select_org implementa o use case SelectOrg.",
		"package select_org",
		`const RegistryKey = "selectOrgService"`,
		"// Service executa o use case SelectOrg.",
	)
}

func TestServiceCreate_CompoundDomain(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "OrgMembership")

	paths, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "OrgMembership", Verb: "Invite"})
	if err != nil {
		t.Fatalf("ServiceCreate: %v", err)
	}
	if want := filepath.Join("internal/application/services/org_membership/invite/service.go"); paths[1] != want {
		t.Errorf("service path: got %q, want %q", paths[1], want)
	}
}

func TestServiceCreate_FailsIfFolderAlreadyExists(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Project")
	if _, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "Project", Verb: "Login"}); err != nil {
		t.Fatalf("primeiro ServiceCreate: %v", err)
	}
	_, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "Project", Verb: "Login"})
	if err == nil {
		t.Fatalf("segundo ServiceCreate: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "já existe") {
		t.Errorf("erro %q não menciona 'já existe'", err.Error())
	}
}

func TestServiceCreate_FailsIfDomainFileMissing(t *testing.T) {
	root := t.TempDir()
	_, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "Missing", Verb: "Login"})
	if err == nil {
		t.Fatalf("ServiceCreate: esperado erro pra domínio inexistente")
	}
}

func TestServiceCreate_FailsIfDomainStructMissing(t *testing.T) {
	root := t.TempDir()
	rel := seedDomain(t, root, "Project")
	if err := os.WriteFile(filepath.Join(root, rel), []byte("package project\n"), 0o600); err != nil {
		t.Fatalf("rewrite domain: %v", err)
	}

	_, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "Project", Verb: "Login"})
	if err == nil {
		t.Fatalf("ServiceCreate: esperado erro pra struct inexistente")
	}
	if !strings.Contains(err.Error(), "Project") {
		t.Errorf("erro %q não menciona Project", err.Error())
	}
}

func TestServiceCreate_InvalidDomainName(t *testing.T) {
	root := t.TempDir()
	_, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "lowercase", Verb: "Login"})
	if err == nil {
		t.Fatalf("ServiceCreate: esperado erro pra Domain inválido")
	}
}

func TestServiceCreate_InvalidVerbName(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Project")
	_, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "Project", Verb: "login"})
	if err == nil {
		t.Fatalf("ServiceCreate: esperado erro pra Verb inválido")
	}
}

func TestServiceCreate_GeneratedFilesAreValidGo(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Project")
	paths, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "Project", Verb: "Login"})
	if err != nil {
		t.Fatalf("ServiceCreate: %v", err)
	}
	for _, rel := range paths {
		src, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if err := parseGoSrc(src); err != nil {
			t.Fatalf("arquivo %s não compila no parser: %v\n%s", rel, err, src)
		}
	}
}

func TestServiceCreate_RerunIsBlocked(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Project")
	if _, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "Project", Verb: "Login"}); err != nil {
		t.Fatalf("primeiro ServiceCreate: %v", err)
	}
	_, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "Project", Verb: "Login"})
	if err == nil {
		t.Fatalf("segundo ServiceCreate: esperado erro, got nil")
	}
}

func TestServiceCreate_TwoVerbsInSameDomain(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Project")

	paths1, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "Project", Verb: "Login"})
	if err != nil {
		t.Fatalf("ServiceCreate Login: %v", err)
	}
	paths2, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: "Project", Verb: "Logout"})
	if err != nil {
		t.Fatalf("ServiceCreate Logout: %v", err)
	}
	if paths1[1] == paths2[1] {
		t.Errorf("verbos distintos geraram o mesmo path: %s", paths1[1])
	}
	got1 := readFile(t, filepath.Join(root, paths1[1]))
	got2 := readFile(t, filepath.Join(root, paths2[1]))
	mustContain(t, got1, "package login", `"loginService"`)
	mustContain(t, got2, "package logout", `"logoutService"`)
}
