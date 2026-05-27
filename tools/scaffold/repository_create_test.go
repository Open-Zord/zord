package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryCreate_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Foo")

	rel, err := RepositoryCreate(RepositoryCreateOptions{Root: root, Domain: "Foo"})
	if err != nil {
		t.Fatalf("RepositoryCreate: %v", err)
	}
	if want := filepath.Join("internal/repositories/foo/foo.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		"package foo_repository",
		`"github.com/jmoiron/sqlx"`,
		`"zord/internal/application/domain/foo"`,
		`"zord/internal/repositories/base_repository"`,
		`const RegistryKey = "fooRepository"`,
		"type FooRepository struct {",
		"*base_repository.BaseRepo[foo.Foo]",
		"func NewFooRepository(mysql *sqlx.DB) *FooRepository",
		"BaseRepo: base_repository.NewBaseRepository[foo.Foo](mysql),",
	)
}

func TestRepositoryCreate_SnakeCaseDomain(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "OrgMembership")

	rel, err := RepositoryCreate(RepositoryCreateOptions{Root: root, Domain: "OrgMembership"})
	if err != nil {
		t.Fatalf("RepositoryCreate: %v", err)
	}
	if want := filepath.Join("internal/repositories/org_membership/org_membership.go"); rel != want {
		t.Errorf("path: got %q, want %q", rel, want)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		"package org_membership_repository",
		`"zord/internal/application/domain/org_membership"`,
		`const RegistryKey = "orgMembershipRepository"`,
		"*base_repository.BaseRepo[org_membership.OrgMembership]",
		"func NewOrgMembershipRepository(mysql *sqlx.DB) *OrgMembershipRepository",
	)
}

func TestRepositoryCreate_FailsIfAlreadyExists(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Foo")
	if _, err := RepositoryCreate(RepositoryCreateOptions{Root: root, Domain: "Foo"}); err != nil {
		t.Fatalf("primeiro RepositoryCreate: %v", err)
	}
	_, err := RepositoryCreate(RepositoryCreateOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("segundo RepositoryCreate: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "já existe") {
		t.Errorf("erro %q não menciona 'já existe'", err.Error())
	}
}

func TestRepositoryCreate_FailsIfDomainFileMissing(t *testing.T) {
	root := t.TempDir()
	_, err := RepositoryCreate(RepositoryCreateOptions{Root: root, Domain: "Missing"})
	if err == nil {
		t.Fatalf("RepositoryCreate: esperado erro pra domínio inexistente")
	}
}

func TestRepositoryCreate_FailsIfDomainStructMissing(t *testing.T) {
	root := t.TempDir()
	rel := seedDomain(t, root, "Foo")
	rewriteDomainFile(t, filepath.Join(root, rel), "package foo\n")

	_, err := RepositoryCreate(RepositoryCreateOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("RepositoryCreate: esperado erro pra struct inexistente")
	}
	if !strings.Contains(err.Error(), "Foo") {
		t.Errorf("erro %q não menciona Foo", err.Error())
	}
}

func TestRepositoryCreate_InvalidDomainName(t *testing.T) {
	root := t.TempDir()
	_, err := RepositoryCreate(RepositoryCreateOptions{Root: root, Domain: "lowercase"})
	if err == nil {
		t.Fatalf("RepositoryCreate: esperado erro pra nome inválido")
	}
}

func TestRepositoryCreate_AfterPortStillWorks(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Foo")
	if _, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Foo"}); err != nil {
		t.Fatalf("RepositoryPort: %v", err)
	}
	rel, err := RepositoryCreate(RepositoryCreateOptions{Root: root, Domain: "Foo"})
	if err != nil {
		t.Fatalf("RepositoryCreate após RepositoryPort: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got, "*base_repository.BaseRepo[foo.Foo]")
}

func TestRepositoryCreate_EmitsRegistryKeyConst(t *testing.T) {
	// NAVE-106: o arquivo gerado precisa expor `const RegistryKey = "<lowerCamel>Repository"`
	// pra `scaffold repository register` conseguir referenciá-lo no map central.
	cases := []struct {
		domain string
		want   string
	}{
		{"Foo", `const RegistryKey = "fooRepository"`},
		{"OrgMembership", `const RegistryKey = "orgMembershipRepository"`},
		{"UserOrgRole", `const RegistryKey = "userOrgRoleRepository"`},
	}
	for _, c := range cases {
		t.Run(c.domain, func(t *testing.T) {
			root := t.TempDir()
			seedDomain(t, root, c.domain)
			rel, err := RepositoryCreate(RepositoryCreateOptions{Root: root, Domain: c.domain})
			if err != nil {
				t.Fatalf("RepositoryCreate: %v", err)
			}
			got := readFile(t, filepath.Join(root, rel))
			if !strings.Contains(got, c.want) {
				t.Errorf("arquivo gerado pra %s não contém %q\n---\n%s", c.domain, c.want, got)
			}
		})
	}
}

func TestRepositoryCreate_GeneratedFileIsValidGo(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Foo")
	rel, err := RepositoryCreate(RepositoryCreateOptions{Root: root, Domain: "Foo"})
	if err != nil {
		t.Fatalf("RepositoryCreate: %v", err)
	}
	src, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := parseGoSrc(src); err != nil {
		t.Fatalf("arquivo gerado não compila no parser: %v\n%s", err, src)
	}
}
