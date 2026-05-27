package scaffold

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parseGoSrc(src []byte) error {
	_, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution)
	return err
}

func TestRepositoryPort_HappyPath(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Foo")

	rel, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Foo"})
	if err != nil {
		t.Fatalf("RepositoryPort: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))

	mustContain(t, got,
		`"zord/internal/application/providers/filters"`,
		`"zord/internal/repositories/base_repository"`,
		"filters *filters.Filters",
		"func (f *Foo) SetFilters(",
		"func (f Foo) SoftDelete() string",
		`return "deleted_at"`,
		"func (f Foo) GetFilters() filters.Filters",
		"if f.filters != nil",
		"return *f.filters",
		"return filters.Filters{}",
		"func (f Foo) Schema() string",
		`return "foos"`,
		"type Repository interface",
		"base_repository.BaseRepository[Foo]",
	)
	mustNotContain(t, got, "client string", "SetClient(")
}

func TestRepositoryPort_MultiTenant(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Foo")

	rel, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Foo", MultiTenant: true})
	if err != nil {
		t.Fatalf("RepositoryPort: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))

	mustContain(t, got,
		"client  string",
		"filters *filters.Filters",
		"func (f *Foo) SetClient(client string)",
		"f.client = client",
		"if f.client ==",
		`return "foos"`,
		`return f.client + "." + "foos"`,
	)
}

func TestRepositoryPort_TableOverride(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "OrgMembership")

	rel, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "OrgMembership", Table: "memberships"})
	if err != nil {
		t.Fatalf("RepositoryPort: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got, `return "memberships"`)
	mustNotContain(t, got, `return "org_memberships"`)
}

func TestRepositoryPort_TableDefaultIsSnakePlusS(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "OrgMembership")

	rel, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "OrgMembership"})
	if err != nil {
		t.Fatalf("RepositoryPort: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got, `return "org_memberships"`)
}

func TestRepositoryPort_FailsIfMethodAlreadyExists(t *testing.T) {
	cases := []struct {
		name      string
		extraSrc  string
		wantInErr string
	}{
		{
			name:      "Schema",
			extraSrc:  "\nfunc (f Foo) Schema() string { return \"x\" }\n",
			wantInErr: "Schema",
		},
		{
			name:      "GetFilters",
			extraSrc:  "\nimport \"zord/internal/application/providers/filters\"\nfunc (f Foo) GetFilters() filters.Filters { return filters.Filters{} }\n",
			wantInErr: "GetFilters",
		},
		{
			name:      "SoftDelete",
			extraSrc:  "\nfunc (f Foo) SoftDelete() string { return \"\" }\n",
			wantInErr: "SoftDelete",
		},
		{
			name:      "SetFilters",
			extraSrc:  "\nimport \"zord/internal/application/providers/filters\"\nfunc (f *Foo) SetFilters(_ *filters.Filters) {}\n",
			wantInErr: "SetFilters",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			rel := seedDomain(t, root, "Foo")
			appendToDomainFile(t, filepath.Join(root, rel), tc.extraSrc)

			before := readFile(t, filepath.Join(root, rel))
			_, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Foo"})
			if err == nil {
				t.Fatalf("RepositoryPort: esperado erro, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Errorf("erro %q não contém %q", err.Error(), tc.wantInErr)
			}
			if got := readFile(t, filepath.Join(root, rel)); got != before {
				t.Errorf("RepositoryPort modificou o arquivo apesar do erro:\n--- before ---\n%s\n--- after ---\n%s", before, got)
			}
		})
	}
}

func TestRepositoryPort_FailsIfSetClientAlreadyExistsMultiTenant(t *testing.T) {
	root := t.TempDir()
	rel := seedDomain(t, root, "Foo")
	appendToDomainFile(t, filepath.Join(root, rel), "\nfunc (f *Foo) SetClient(_ string) {}\n")

	_, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Foo", MultiTenant: true})
	if err == nil {
		t.Fatalf("RepositoryPort: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "SetClient") {
		t.Errorf("erro %q não menciona SetClient", err.Error())
	}
}

func TestRepositoryPort_FailsIfFiltersFieldAlreadyExists(t *testing.T) {
	root := t.TempDir()
	rel := seedDomain(t, root, "Foo")
	rewriteDomainFile(t, filepath.Join(root, rel),
		"package foo\n\ntype Foo struct {\n\tfilters int\n}\n")

	_, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("RepositoryPort: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "filters") {
		t.Errorf("erro %q não menciona filters", err.Error())
	}
}

func TestRepositoryPort_FailsIfClientFieldAlreadyExistsMultiTenant(t *testing.T) {
	root := t.TempDir()
	rel := seedDomain(t, root, "Foo")
	rewriteDomainFile(t, filepath.Join(root, rel),
		"package foo\n\ntype Foo struct {\n\tclient string\n}\n")

	_, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Foo", MultiTenant: true})
	if err == nil {
		t.Fatalf("RepositoryPort: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "client") {
		t.Errorf("erro %q não menciona client", err.Error())
	}
}

func TestRepositoryPort_FailsIfRepositoryInterfaceAlreadyExists(t *testing.T) {
	root := t.TempDir()
	rel := seedDomain(t, root, "Foo")
	appendToDomainFile(t, filepath.Join(root, rel), "\ntype Repository interface{}\n")

	_, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("RepositoryPort: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "Repository") {
		t.Errorf("erro %q não menciona Repository", err.Error())
	}
}

func TestRepositoryPort_FailsIfStructMissing(t *testing.T) {
	root := t.TempDir()
	rel := seedDomain(t, root, "Foo")
	rewriteDomainFile(t, filepath.Join(root, rel), "package foo\n")

	_, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("RepositoryPort: esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "Foo") {
		t.Errorf("erro %q não menciona Foo", err.Error())
	}
}

func TestRepositoryPort_FailsIfDomainFileMissing(t *testing.T) {
	root := t.TempDir()
	_, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Missing"})
	if err == nil {
		t.Fatalf("RepositoryPort: esperado erro pra domínio inexistente")
	}
}

func TestRepositoryPort_RerunIsBlocked(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Foo")
	if _, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Foo"}); err != nil {
		t.Fatalf("RepositoryPort primeira: %v", err)
	}
	_, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("RepositoryPort segunda: esperado erro de duplicação, got nil")
	}
}

func TestRepositoryPort_InvalidDomainName(t *testing.T) {
	root := t.TempDir()
	_, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "lowercase"})
	if err == nil {
		t.Fatalf("RepositoryPort: esperado erro pra nome inválido")
	}
}

func appendToDomainFile(t *testing.T, path, extra string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(b, []byte(extra)...), 0o600); err != nil {
		t.Fatalf("append %s: %v", path, err)
	}
}

func rewriteDomainFile(t *testing.T, path, src string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("rewrite %s: %v", path, err)
	}
}
