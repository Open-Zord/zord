package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDomainCreate_HappyPath(t *testing.T) {
	root := t.TempDir()
	rel, err := DomainCreate("OrgMembership", DomainCreateOptions{Root: root})
	if err != nil {
		t.Fatalf("DomainCreate: %v", err)
	}
	want := filepath.Join(BasePath, "org_membership", "org_membership.go")
	if rel != want {
		t.Fatalf("rel = %q; want %q", rel, want)
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	got := string(body)
	if !strings.Contains(got, "package org_membership") {
		t.Errorf("missing package decl; got:\n%s", got)
	}
	if !strings.Contains(got, "type OrgMembership struct") {
		t.Errorf("missing type decl; got:\n%s", got)
	}
}

func TestDomainCreate_Duplicate(t *testing.T) {
	root := t.TempDir()
	if _, err := DomainCreate("Foo", DomainCreateOptions{Root: root}); err != nil {
		t.Fatalf("first DomainCreate: %v", err)
	}
	if _, err := DomainCreate("Foo", DomainCreateOptions{Root: root}); err == nil {
		t.Fatalf("second DomainCreate: want error, got nil")
	} else if !strings.Contains(err.Error(), "já existe") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDomainCreate_InvalidName(t *testing.T) {
	root := t.TempDir()
	cases := []string{"", "foo", "1Foo", "Foo-Bar"}
	for _, n := range cases {
		if _, err := DomainCreate(n, DomainCreateOptions{Root: root}); err == nil {
			t.Errorf("DomainCreate(%q): want error, got nil", n)
		}
	}
}
