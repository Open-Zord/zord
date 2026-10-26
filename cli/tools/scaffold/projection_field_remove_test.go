package scaffold

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectionFieldRemove_HappyPath(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "Summary")

	if _, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
		FieldName: "Total", TypeStr: "float64", NoDBTag: true,
	}); err != nil {
		t.Fatalf("seed ProjectionFieldAdd: %v", err)
	}

	rel, err := ProjectionFieldRemove(ProjectionFieldRemoveOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary", FieldName: "Total",
	})
	if err != nil {
		t.Fatalf("ProjectionFieldRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	if strings.Contains(got, "Total") {
		t.Errorf("field still present:\n%s", got)
	}
}

func TestProjectionFieldRemove_MissingField(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "Summary")

	_, err := ProjectionFieldRemove(ProjectionFieldRemoveOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary", FieldName: "Ghost",
	})
	if err == nil {
		t.Fatalf("ProjectionFieldRemove: want error, got nil")
	}
	if !strings.Contains(err.Error(), "não existe") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProjectionFieldRemove_MissingProjection(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")

	_, err := ProjectionFieldRemove(ProjectionFieldRemoveOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Missing", FieldName: "X",
	})
	if err == nil {
		t.Fatalf("ProjectionFieldRemove: want error for missing projection")
	}
}

func TestProjectionFieldRemove_PrunesUnusedImport(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "Summary")

	if _, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
		FieldName: "PeriodStart", TypeStr: "time.Time", NoDBTag: true,
	}); err != nil {
		t.Fatalf("seed ProjectionFieldAdd: %v", err)
	}
	if _, err := ProjectionFieldRemove(ProjectionFieldRemoveOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary", FieldName: "PeriodStart",
	}); err != nil {
		t.Fatalf("ProjectionFieldRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, "internal/application/domain/usage_record/usage_record.go"))
	if strings.Contains(got, `"time"`) {
		t.Errorf("time import not pruned:\n%s", got)
	}
}

func TestProjectionFieldRemove_KeepsImportUsedElsewhere(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "Summary")
	newProjection(t, root, "UsageRecord", "Other")

	if _, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
		FieldName: "PeriodStart", TypeStr: "time.Time", NoDBTag: true,
	}); err != nil {
		t.Fatalf("seed Summary.PeriodStart: %v", err)
	}
	if _, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Other",
		FieldName: "PeriodEnd", TypeStr: "time.Time", NoDBTag: true,
	}); err != nil {
		t.Fatalf("seed Other.PeriodEnd: %v", err)
	}
	if _, err := ProjectionFieldRemove(ProjectionFieldRemoveOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary", FieldName: "PeriodStart",
	}); err != nil {
		t.Fatalf("ProjectionFieldRemove: %v", err)
	}
	got := readFile(t, filepath.Join(root, "internal/application/domain/usage_record/usage_record.go"))
	if !strings.Contains(got, `"time"`) {
		t.Errorf("time import wrongly pruned (Other.PeriodEnd still uses it):\n%s", got)
	}
}
