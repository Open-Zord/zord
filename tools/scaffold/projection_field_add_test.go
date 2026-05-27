package scaffold

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// normalizeWS colapsa runs de whitespace (espaços/tabs) em um único espaço
// pra permitir asserts insensíveis ao alinhamento de colunas do gofmt.
func normalizeWS(s string) string {
	return regexp.MustCompile(`[ \t]+`).ReplaceAllString(s, " ")
}

func newProjection(t *testing.T, root, domain, projection string) {
	t.Helper()
	if _, err := ProjectionCreate(ProjectionCreateOptions{
		Root: root, Domain: domain, ProjectionName: projection,
	}); err != nil {
		t.Fatalf("seed projection %s/%s: %v", domain, projection, err)
	}
}

func TestProjectionFieldAdd_ScalarFieldWithDefaults(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "ResourceSummary")

	rel, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "ResourceSummary",
		FieldName: "ResourceType", TypeStr: "string",
	})
	if err != nil {
		t.Fatalf("ProjectionFieldAdd: %v", err)
	}
	got := normalizeWS(readFile(t, filepath.Join(root, rel)))
	want := "ResourceType string `db:\"resource_type\" json:\"resource_type\"`"
	if !strings.Contains(got, want) {
		t.Errorf("missing %q in:\n%s", want, got)
	}
}

func TestProjectionFieldAdd_DBTagOverride(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "ResourceSummary")

	rel, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "ResourceSummary",
		FieldName: "Total", TypeStr: "float64",
		DBTagSet: true, DBTagValue: "total_cost",
	})
	if err != nil {
		t.Fatalf("ProjectionFieldAdd: %v", err)
	}
	got := normalizeWS(readFile(t, filepath.Join(root, rel)))
	want := "Total float64 `db:\"total_cost\" json:\"total\"`"
	if !strings.Contains(got, want) {
		t.Errorf("missing %q in:\n%s", want, got)
	}
}

func TestProjectionFieldAdd_NoDBTag(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "Summary")

	rel, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
		FieldName: "Total", TypeStr: "float64",
		NoDBTag: true,
	})
	if err != nil {
		t.Fatalf("ProjectionFieldAdd: %v", err)
	}
	got := normalizeWS(readFile(t, filepath.Join(root, rel)))
	want := "Total float64 `json:\"total\"`"
	if !strings.Contains(got, want) {
		t.Errorf("missing %q in:\n%s", want, got)
	}
	if strings.Contains(got, "db:\"total\"") {
		t.Errorf("db tag should be absent; got:\n%s", got)
	}
}

func TestProjectionFieldAdd_PointerType_AddsImport(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "Summary")

	rel, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
		FieldName: "PeriodStart", TypeStr: "*time.Time",
	})
	if err != nil {
		t.Fatalf("ProjectionFieldAdd: %v", err)
	}
	got := normalizeWS(readFile(t, filepath.Join(root, rel)))
	if !strings.Contains(got, `"time"`) {
		t.Errorf("time import missing; got:\n%s", got)
	}
	if !strings.Contains(got, "PeriodStart *time.Time") {
		t.Errorf("missing pointer field; got:\n%s", got)
	}
}

func TestProjectionFieldAdd_SliceOfProjection_NoDBTagAutomatic(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "ResourceSummary")
	newProjection(t, root, "UsageRecord", "Summary")

	rel, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
		FieldName: "ByResource", TypeStr: "[]ResourceSummary",
	})
	if err != nil {
		t.Fatalf("ProjectionFieldAdd: %v", err)
	}
	got := normalizeWS(readFile(t, filepath.Join(root, rel)))
	want := "ByResource []ResourceSummary `json:\"by_resource\"`"
	if !strings.Contains(got, want) {
		t.Errorf("missing %q in:\n%s", want, got)
	}
	if strings.Contains(got, "db:\"by_resource\"") {
		t.Errorf("db tag should be absent for slice; got:\n%s", got)
	}
}

func TestProjectionFieldAdd_SliceOfMissingProjection_Fails(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "Summary")

	_, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
		FieldName: "ByResource", TypeStr: "[]ResourceSummary",
	})
	if err == nil {
		t.Fatalf("ProjectionFieldAdd: want error for missing referenced projection")
	}
	if !strings.Contains(err.Error(), "ResourceSummary") {
		t.Errorf("error should mention missing type ResourceSummary; got: %v", err)
	}
}

func TestProjectionFieldAdd_Duplicate(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "Summary")

	if _, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
		FieldName: "Total", TypeStr: "float64",
	}); err != nil {
		t.Fatalf("first ProjectionFieldAdd: %v", err)
	}
	_, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
		FieldName: "Total", TypeStr: "float64",
	})
	if err == nil {
		t.Fatalf("second ProjectionFieldAdd: want error")
	}
	if !strings.Contains(err.Error(), "já existe") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProjectionFieldAdd_MissingProjection(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")

	_, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Missing",
		FieldName: "X", TypeStr: "string",
	})
	if err == nil {
		t.Fatalf("ProjectionFieldAdd: want error for missing projection")
	}
}

func TestProjectionFieldAdd_DBTagAndNoDBTag_MutuallyExclusive(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "Summary")

	_, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
		FieldName: "Total", TypeStr: "float64",
		DBTagSet: true, DBTagValue: "x", NoDBTag: true,
	})
	if err == nil {
		t.Fatalf("ProjectionFieldAdd: want error for conflicting flags")
	}
}

func TestProjectionFieldAdd_ReproducesNAVE9Pattern(t *testing.T) {
	// E2E: reproduz ResourceSummary + Summary do NAVE-9 (usage_record).
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	newProjection(t, root, "UsageRecord", "ResourceSummary")
	newProjection(t, root, "UsageRecord", "Summary")

	rsFields := []struct {
		name, typ string
	}{
		{"ResourceType", "string"},
		{"Unit", "string"},
		{"Quantity", "float64"},
		{"Total", "float64"},
	}
	for _, f := range rsFields {
		if _, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
			Root: root, Domain: "UsageRecord", ProjectionName: "ResourceSummary",
			FieldName: f.name, TypeStr: f.typ,
		}); err != nil {
			t.Fatalf("ResourceSummary.%s: %v", f.name, err)
		}
	}

	type sf struct {
		name, typ string
		noDB      bool
	}
	sumFields := []sf{
		{"Namespace", "string", true},
		{"PeriodStart", "string", true},
		{"PeriodEnd", "string", true},
		{"Total", "float64", true},
		{"ByResource", "[]ResourceSummary", false},
	}
	for _, f := range sumFields {
		if _, err := ProjectionFieldAdd(ProjectionFieldAddOptions{
			Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
			FieldName: f.name, TypeStr: f.typ, NoDBTag: f.noDB,
		}); err != nil {
			t.Fatalf("Summary.%s: %v", f.name, err)
		}
	}

	got := normalizeWS(readFile(t, filepath.Join(root, "internal/application/domain/usage_record/usage_record.go")))
	wants := []string{
		"type ResourceSummary struct",
		"ResourceType string `db:\"resource_type\" json:\"resource_type\"`",
		"Unit string `db:\"unit\" json:\"unit\"`",
		"Quantity float64 `db:\"quantity\" json:\"quantity\"`",
		"Total float64 `db:\"total\" json:\"total\"`",
		"type Summary struct",
		"Namespace string `json:\"namespace\"`",
		"PeriodEnd string `json:\"period_end\"`",
		"ByResource []ResourceSummary `json:\"by_resource\"`",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in final file:\n%s", w, got)
		}
	}
}
