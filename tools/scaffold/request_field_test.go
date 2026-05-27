package scaffold

import (
	"path/filepath"
	"strings"
	"testing"
)

// seedDomainAndService cria domain + service trio pra alimentar os testes
// de request/response field.
func seedDomainAndService(t *testing.T, root, domain, verb string) {
	t.Helper()
	seedDomain(t, root, domain)
	if _, err := ServiceCreate(ServiceCreateOptions{Root: root, Domain: domain, Verb: verb}); err != nil {
		t.Fatalf("ServiceCreate %s/%s: %v", domain, verb, err)
	}
}

func TestRequestFieldAdd_SimpleField(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")

	rel, err := RequestFieldAdd(RequestFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "Name", TypeStr: "string",
	})
	if err != nil {
		t.Fatalf("RequestFieldAdd: %v", err)
	}
	if want := filepath.Join("internal/application/services/foo/create/request.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		"Name string",
		"`json:\"name\"`",
	)
}

func TestRequestFieldAdd_WithValidate(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")

	rel, err := RequestFieldAdd(RequestFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create",
		FieldName: "Email", TypeStr: "string", Validate: "required,email",
	})
	if err != nil {
		t.Fatalf("RequestFieldAdd: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got, "Email string", "`json:\"email\" validate:\"required,email\"`")
}

func TestRequestFieldAdd_CompoundFieldName(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")

	rel, err := RequestFieldAdd(RequestFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create",
		FieldName: "UserID", TypeStr: "string",
	})
	if err != nil {
		t.Fatalf("RequestFieldAdd: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got, "`json:\"user_id\"`")
}

func TestRequestFieldAdd_PointerType(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")

	rel, err := RequestFieldAdd(RequestFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create",
		FieldName: "DeletedAt", TypeStr: "*time.Time",
	})
	if err != nil {
		t.Fatalf("RequestFieldAdd: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got, `"time"`, "DeletedAt *time.Time")
}

func TestRequestFieldAdd_Duplicate(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	if _, err := RequestFieldAdd(RequestFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "Name", TypeStr: "string",
	}); err != nil {
		t.Fatalf("first RequestFieldAdd: %v", err)
	}
	_, err := RequestFieldAdd(RequestFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "Name", TypeStr: "string",
	})
	if err == nil || !strings.Contains(err.Error(), "já existe") {
		t.Fatalf("RequestFieldAdd dup: want 'já existe', got %v", err)
	}
}

func TestRequestFieldAdd_MissingRequestFile(t *testing.T) {
	root := t.TempDir()
	_, err := RequestFieldAdd(RequestFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "Name", TypeStr: "string",
	})
	if err == nil {
		t.Fatalf("RequestFieldAdd: esperado erro pra request inexistente")
	}
}

func TestRequestFieldRemove_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	if _, err := RequestFieldAdd(RequestFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "Name", TypeStr: "string",
	}); err != nil {
		t.Fatalf("RequestFieldAdd: %v", err)
	}
	rel, err := RequestFieldRemove(RequestFieldRemoveOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "Name",
	})
	if err != nil {
		t.Fatalf("RequestFieldRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	if strings.Contains(got, "Name string") {
		t.Errorf("field still present:\n%s", got)
	}
}

func TestRequestFieldRemove_Unknown(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	_, err := RequestFieldRemove(RequestFieldRemoveOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "Ghost",
	})
	if err == nil || !strings.Contains(err.Error(), "não existe") {
		t.Fatalf("RequestFieldRemove: want 'não existe', got %v", err)
	}
}

func TestRequestFieldRemove_PrunesUnusedImport(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	if _, err := RequestFieldAdd(RequestFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "CreatedAt", TypeStr: "time.Time",
	}); err != nil {
		t.Fatalf("RequestFieldAdd: %v", err)
	}
	rel, err := RequestFieldRemove(RequestFieldRemoveOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "CreatedAt",
	})
	if err != nil {
		t.Fatalf("RequestFieldRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	if strings.Contains(got, `"time"`) {
		t.Errorf("time import not pruned:\n%s", got)
	}
}
