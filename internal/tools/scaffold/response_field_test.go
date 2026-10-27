package scaffold

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResponseFieldAdd_SimpleField(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")

	rel, err := ResponseFieldAdd(ResponseFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "ID", TypeStr: "string",
	})
	if err != nil {
		t.Fatalf("ResponseFieldAdd: %v", err)
	}
	if want := filepath.Join("internal/application/services/foo/create/response.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got, "ID string", "`json:\"id\"`")
}

func TestResponseFieldAdd_CompoundFieldName(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	rel, err := ResponseFieldAdd(ResponseFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "CreatedAt", TypeStr: "time.Time",
	})
	if err != nil {
		t.Fatalf("ResponseFieldAdd: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got, `"time"`, "CreatedAt time.Time", "`json:\"created_at\"`")
}

func TestResponseFieldAdd_Duplicate(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	if _, err := ResponseFieldAdd(ResponseFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "ID", TypeStr: "string",
	}); err != nil {
		t.Fatalf("first ResponseFieldAdd: %v", err)
	}
	_, err := ResponseFieldAdd(ResponseFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "ID", TypeStr: "string",
	})
	if err == nil || !strings.Contains(err.Error(), "já existe") {
		t.Fatalf("ResponseFieldAdd dup: want erro 'já existe', got %v", err)
	}
}

func TestResponseFieldRemove_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	if _, err := ResponseFieldAdd(ResponseFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "ID", TypeStr: "string",
	}); err != nil {
		t.Fatalf("ResponseFieldAdd: %v", err)
	}
	rel, err := ResponseFieldRemove(ResponseFieldRemoveOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "ID",
	})
	if err != nil {
		t.Fatalf("ResponseFieldRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	if strings.Contains(got, "ID string") {
		t.Errorf("campo ID ainda presente:\n%s", got)
	}
}

func TestResponseFieldRemove_PrunesUnusedImport(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	if _, err := ResponseFieldAdd(ResponseFieldAddOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "CreatedAt", TypeStr: "time.Time",
	}); err != nil {
		t.Fatalf("ResponseFieldAdd: %v", err)
	}
	rel, err := ResponseFieldRemove(ResponseFieldRemoveOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "CreatedAt",
	})
	if err != nil {
		t.Fatalf("ResponseFieldRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	if strings.Contains(got, `"time"`) {
		t.Errorf("time import not pruned:\n%s", got)
	}
}

func TestResponseFieldRemove_Unknown(t *testing.T) {
	root := t.TempDir()
	seedDomainAndService(t, root, "Foo", "Create")
	_, err := ResponseFieldRemove(ResponseFieldRemoveOptions{
		Root: root, Domain: "Foo", Verb: "Create", FieldName: "Ghost",
	})
	if err == nil || !strings.Contains(err.Error(), "não existe") {
		t.Fatalf("ResponseFieldRemove: want 'não existe', got %v", err)
	}
}
