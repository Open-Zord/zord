package scaffold

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectionCreate_HappyPath(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")

	rel, err := ProjectionCreate(ProjectionCreateOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "ResourceSummary",
	})
	if err != nil {
		t.Fatalf("ProjectionCreate: %v", err)
	}
	want := filepath.Join("internal/application/domain", "usage_record", "usage_record.go")
	if rel != want {
		t.Fatalf("rel = %q; want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	if !strings.Contains(got, "type ResourceSummary struct") {
		t.Errorf("missing projection decl; got:\n%s", got)
	}
	if !strings.Contains(got, "type UsageRecord struct") {
		t.Errorf("domain struct lost; got:\n%s", got)
	}
}

func TestProjectionCreate_MultipleAccumulate(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")

	for _, n := range []string{"ResourceSummary", "Summary"} {
		if _, err := ProjectionCreate(ProjectionCreateOptions{
			Root: root, Domain: "UsageRecord", ProjectionName: n,
		}); err != nil {
			t.Fatalf("ProjectionCreate %s: %v", n, err)
		}
	}
	got := readFile(t, filepath.Join(root, "internal/application/domain/usage_record/usage_record.go"))
	for _, want := range []string{"type ResourceSummary struct", "type Summary struct"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestProjectionCreate_DuplicateProjection(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "UsageRecord")
	if _, err := ProjectionCreate(ProjectionCreateOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
	}); err != nil {
		t.Fatalf("first ProjectionCreate: %v", err)
	}
	_, err := ProjectionCreate(ProjectionCreateOptions{
		Root: root, Domain: "UsageRecord", ProjectionName: "Summary",
	})
	if err == nil {
		t.Fatalf("second ProjectionCreate: want error, got nil")
	}
	if !strings.Contains(err.Error(), "já existe") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProjectionCreate_CollidesWithDomain(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "Foo")
	_, err := ProjectionCreate(ProjectionCreateOptions{
		Root: root, Domain: "Foo", ProjectionName: "Foo",
	})
	if err == nil {
		t.Fatalf("ProjectionCreate: want error for self-collision")
	}
	if !strings.Contains(err.Error(), "mesmo nome do domínio") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProjectionCreate_InvalidNames(t *testing.T) {
	root := t.TempDir()
	newDomain(t, root, "Foo")
	cases := []struct {
		domain, name string
	}{
		{"", "Summary"},
		{"foo", "Summary"},
		{"Foo", ""},
		{"Foo", "summary"},
		{"Foo", "1Summary"},
	}
	for _, c := range cases {
		if _, err := ProjectionCreate(ProjectionCreateOptions{
			Root: root, Domain: c.domain, ProjectionName: c.name,
		}); err == nil {
			t.Errorf("ProjectionCreate(domain=%q, name=%q): want error, got nil", c.domain, c.name)
		}
	}
}

func TestProjectionCreate_MissingDomain(t *testing.T) {
	root := t.TempDir()
	_, err := ProjectionCreate(ProjectionCreateOptions{
		Root: root, Domain: "Missing", ProjectionName: "Summary",
	})
	if err == nil {
		t.Fatalf("ProjectionCreate: want error for missing domain")
	}
}
