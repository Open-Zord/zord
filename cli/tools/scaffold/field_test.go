package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newDomain(t *testing.T, root, typeName string) string {
	t.Helper()
	rel, err := DomainCreate(typeName, DomainCreateOptions{Root: root})
	if err != nil {
		t.Fatalf("seed domain %s: %v", typeName, err)
	}
	return rel
}

func TestFieldAdd_SimpleField(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "Foo")

	rel, err := FieldAdd(FieldAddOptions{
		Root: root, Domain: "Foo", FieldName: "Name", TypeStr: "string",
	})
	if err != nil {
		t.Fatalf("FieldAdd: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	if !strings.Contains(got, "Name string") {
		t.Errorf("missing field; got:\n%s", got)
	}
}

func TestFieldAdd_MultipleTagsCanonicalOrder(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "Foo")

	tags := []FieldTag{
		{Key: "db", Value: "user_id"},
		{Key: "json", Value: "user_id"},
		{Key: "validate", Value: "required,uuid"},
		{Key: "db_pk", Value: ""},
	}
	rel, err := FieldAdd(FieldAddOptions{
		Root: root, Domain: "Foo", FieldName: "UserID", TypeStr: "string", Tags: tags,
	})
	if err != nil {
		t.Fatalf("FieldAdd: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	want := "`db:\"user_id\" json:\"user_id\" validate:\"required,uuid\" db_pk:\"\"`"
	if !strings.Contains(got, want) {
		t.Errorf("missing canonical tag string %q in:\n%s", want, got)
	}
}

func TestFieldAdd_PointerType(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "Foo")

	rel, err := FieldAdd(FieldAddOptions{
		Root: root, Domain: "Foo", FieldName: "DeletedAt", TypeStr: "*time.Time",
		Tags: []FieldTag{{Key: "db", Value: "deleted_at"}},
	})
	if err != nil {
		t.Fatalf("FieldAdd: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	if !strings.Contains(got, `"time"`) {
		t.Errorf("import \"time\" missing; got:\n%s", got)
	}
	if !strings.Contains(got, "DeletedAt *time.Time") {
		t.Errorf("missing pointer field; got:\n%s", got)
	}
}

func TestFieldAdd_Duplicate(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "Foo")
	if _, err := FieldAdd(FieldAddOptions{Root: root, Domain: "Foo", FieldName: "Name", TypeStr: "string"}); err != nil {
		t.Fatalf("first FieldAdd: %v", err)
	}
	_, err := FieldAdd(FieldAddOptions{Root: root, Domain: "Foo", FieldName: "Name", TypeStr: "string"})
	if err == nil {
		t.Fatalf("second FieldAdd: want error, got nil")
	}
	if !strings.Contains(err.Error(), "já existe") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFieldAdd_MultipleAddsAccumulate(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "Foo")
	for _, f := range []string{"A", "B", "C"} {
		if _, err := FieldAdd(FieldAddOptions{Root: root, Domain: "Foo", FieldName: f, TypeStr: "string"}); err != nil {
			t.Fatalf("FieldAdd %s: %v", f, err)
		}
	}
	got := readFile(t, filepath.Join(root, "internal/application/domain/foo/foo.go"))
	for _, want := range []string{"A string", "B string", "C string"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestFieldAdd_UnknownDomain(t *testing.T) {
	root := t.TempDir()
	if _, err := FieldAdd(FieldAddOptions{Root: root, Domain: "Missing", FieldName: "X", TypeStr: "string"}); err == nil {
		t.Fatalf("FieldAdd: want error for missing domain")
	}
}

func TestFieldAdd_UnknownImport(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "Foo")
	_, err := FieldAdd(FieldAddOptions{Root: root, Domain: "Foo", FieldName: "X", TypeStr: "foo.Bar"})
	if err == nil {
		t.Fatalf("FieldAdd: want error for unknown package")
	}
	if !strings.Contains(err.Error(), "não suportado") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFieldRemove_HappyPath(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "Foo")
	if _, err := FieldAdd(FieldAddOptions{Root: root, Domain: "Foo", FieldName: "Name", TypeStr: "string"}); err != nil {
		t.Fatalf("FieldAdd: %v", err)
	}
	rel, err := FieldRemove(FieldRemoveOptions{Root: root, Domain: "Foo", FieldName: "Name"})
	if err != nil {
		t.Fatalf("FieldRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	if strings.Contains(got, "Name") {
		t.Errorf("field still present:\n%s", got)
	}
}

func TestFieldRemove_Unknown(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "Foo")
	_, err := FieldRemove(FieldRemoveOptions{Root: root, Domain: "Foo", FieldName: "Ghost"})
	if err == nil {
		t.Fatalf("FieldRemove: want error, got nil")
	}
	if !strings.Contains(err.Error(), "não existe") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFieldRemove_PrunesUnusedImport(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "Foo")
	if _, err := FieldAdd(FieldAddOptions{Root: root, Domain: "Foo", FieldName: "CreatedAt", TypeStr: "time.Time"}); err != nil {
		t.Fatalf("FieldAdd: %v", err)
	}
	if _, err := FieldRemove(FieldRemoveOptions{Root: root, Domain: "Foo", FieldName: "CreatedAt"}); err != nil {
		t.Fatalf("FieldRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, "internal/application/domain/foo/foo.go"))
	if strings.Contains(got, `"time"`) {
		t.Errorf("time import not pruned:\n%s", got)
	}
}

func TestFieldRemove_KeepsUsedImport(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "Foo")
	if _, err := FieldAdd(FieldAddOptions{Root: root, Domain: "Foo", FieldName: "CreatedAt", TypeStr: "time.Time"}); err != nil {
		t.Fatalf("FieldAdd CreatedAt: %v", err)
	}
	if _, err := FieldAdd(FieldAddOptions{Root: root, Domain: "Foo", FieldName: "UpdatedAt", TypeStr: "time.Time"}); err != nil {
		t.Fatalf("FieldAdd UpdatedAt: %v", err)
	}
	if _, err := FieldRemove(FieldRemoveOptions{Root: root, Domain: "Foo", FieldName: "CreatedAt"}); err != nil {
		t.Fatalf("FieldRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, "internal/application/domain/foo/foo.go"))
	if !strings.Contains(got, `"time"`) {
		t.Errorf("time import wrongly pruned (UpdatedAt still uses it):\n%s", got)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
