package scaffold

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// parseOrFail é um helper local para testes que precisam do *ast.File parsed.
func parseOrFail(t *testing.T, src string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, src)
	}
	return f
}

// portFoo é um helper: cria domínio Foo e roda RepositoryPort (single ou multi-
// tenant), retornando o path relativo do arquivo do domínio.
func portFoo(t *testing.T, root, domain string, multiTenant bool) string {
	t.Helper()
	rel := seedDomain(t, root, domain)
	_, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: domain, MultiTenant: multiTenant})
	if err != nil {
		t.Fatalf("seed RepositoryPort %s (multi=%v): %v", domain, multiTenant, err)
	}
	return rel
}

func TestRepositoryUnport_HappyPathSingleTenant(t *testing.T) {
	root := t.TempDir()
	rel := portFoo(t, root, "Foo", false)

	out, err := RepositoryUnport(RepositoryUnportOptions{Root: root, Domain: "Foo"})
	if err != nil {
		t.Fatalf("RepositoryUnport: %v", err)
	}
	if out != rel {
		t.Errorf("retornou %q, esperado %q", out, rel)
	}

	got := readFile(t, filepath.Join(root, rel))
	if err := parseGoSrc([]byte(got)); err != nil {
		t.Fatalf("arquivo não compila no parser: %v\n%s", err, got)
	}
	mustNotContain(t, got,
		"func (f Foo) Schema()",
		"func (f Foo) GetFilters()",
		"func (f Foo) SoftDelete()",
		"func (f *Foo) SetFilters(",
		"filters *filters.Filters",
		"type Repository",
		"zord/internal/application/providers/filters",
		"zord/internal/repositories/base_repository",
	)
}

func TestRepositoryUnport_HappyPathMultiTenant(t *testing.T) {
	root := t.TempDir()
	rel := portFoo(t, root, "Foo", true)

	_, err := RepositoryUnport(RepositoryUnportOptions{Root: root, Domain: "Foo"})
	if err != nil {
		t.Fatalf("RepositoryUnport: %v", err)
	}

	got := readFile(t, filepath.Join(root, rel))
	if err := parseGoSrc([]byte(got)); err != nil {
		t.Fatalf("arquivo não compila no parser: %v\n%s", err, got)
	}
	mustNotContain(t, got,
		"func (f *Foo) SetClient(",
		"client string",
		"client  string",
		"filters *filters.Filters",
		"type Repository",
		"zord/internal/application/providers/filters",
		"zord/internal/repositories/base_repository",
	)
}

func TestRepositoryUnport_AutoDetectsMultiTenant(t *testing.T) {
	// Se rodar como single-tenant em arquivo multi-tenant, sobrariam client e
	// SetClient. Garantia: unport remove tudo sem flag explícita.
	root := t.TempDir()
	rel := portFoo(t, root, "Foo", true)

	if _, err := RepositoryUnport(RepositoryUnportOptions{Root: root, Domain: "Foo"}); err != nil {
		t.Fatalf("RepositoryUnport: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	if strings.Contains(got, "SetClient") {
		t.Errorf("auto-detect falhou: SetClient residual\n%s", got)
	}
	if strings.Contains(got, "client") && strings.Contains(got, "string") &&
		strings.Contains(got, "\tclient ") {
		t.Errorf("auto-detect falhou: campo client residual\n%s", got)
	}
}

func TestRepositoryUnport_PreservesImportsStillUsed(t *testing.T) {
	root := t.TempDir()
	rel := portFoo(t, root, "Foo", false)

	// Adiciona uma função extra usando `filters.Filters` — o import precisa
	// sobreviver ao prune.
	appendToDomainFile(t, filepath.Join(root, rel),
		"\nfunc UsedFilter() filters.Filters { return filters.Filters{} }\n")

	if _, err := RepositoryUnport(RepositoryUnportOptions{Root: root, Domain: "Foo"}); err != nil {
		t.Fatalf("RepositoryUnport: %v", err)
	}
	got := readFile(t, filepath.Join(root, rel))
	mustContain(t, got,
		`"zord/internal/application/providers/filters"`,
		"func UsedFilter() filters.Filters",
	)
	mustNotContain(t, got,
		`"zord/internal/repositories/base_repository"`,
		"type Repository",
	)
}

func TestRepositoryUnport_FailsIfNotPorted(t *testing.T) {
	root := t.TempDir()
	seedDomain(t, root, "Foo")
	// domínio existe mas nunca foi portado.

	_, err := RepositoryUnport(RepositoryUnportOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro pra domínio não-portado")
	}
}

func TestRepositoryUnport_FailsIfMissingElement(t *testing.T) {
	// Porta o domínio e depois remove à mão UM elemento por vez antes do unport.
	// Cada caso deve falhar sem mutar o arquivo restante.
	cases := []struct {
		name      string
		mutate    func(t *testing.T, root, rel string)
		wantInErr string
	}{
		{
			name: "missing Schema",
			mutate: func(t *testing.T, root, rel string) {
				path := filepath.Join(root, rel)
				src := readFile(t, path)
				idx := strings.Index(src, "func (f Foo) Schema()")
				if idx < 0 {
					t.Fatalf("seed inválido")
				}
				end := strings.Index(src[idx:], "\n}")
				rewriteDomainFile(t, path, src[:idx]+src[idx+end+2:])
			},
			wantInErr: "Schema",
		},
		{
			name: "missing filters field",
			mutate: func(t *testing.T, root, rel string) {
				path := filepath.Join(root, rel)
				src := readFile(t, path)
				rewriteDomainFile(t, path, strings.Replace(src, "filters *filters.Filters", "", 1))
			},
			wantInErr: "filters",
		},
		{
			name: "missing Repository interface",
			mutate: func(t *testing.T, root, rel string) {
				path := filepath.Join(root, rel)
				src := readFile(t, path)
				idx := strings.Index(src, "type Repository interface")
				if idx < 0 {
					t.Fatalf("seed inválido")
				}
				end := strings.Index(src[idx:], "\n}")
				rewriteDomainFile(t, path, src[:idx]+src[idx+end+2:])
			},
			wantInErr: "Repository",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			rel := portFoo(t, root, "Foo", false)
			tc.mutate(t, root, rel)

			before := readFile(t, filepath.Join(root, rel))
			_, err := RepositoryUnport(RepositoryUnportOptions{Root: root, Domain: "Foo"})
			if err == nil {
				t.Fatalf("esperado erro, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Errorf("erro %q não menciona %q", err.Error(), tc.wantInErr)
			}
			after := readFile(t, filepath.Join(root, rel))
			if before != after {
				t.Errorf("RepositoryUnport mutou apesar do erro:\n--- before ---\n%s\n--- after ---\n%s", before, after)
			}
		})
	}
}

func TestRepositoryUnport_FailsIfPartialMultiTenant(t *testing.T) {
	// Porta single-tenant; injeta apenas SetClient à mão. Estado deve ser erro
	// "inconsistente" (campo ausente, método presente).
	root := t.TempDir()
	rel := portFoo(t, root, "Foo", false)
	appendToDomainFile(t, filepath.Join(root, rel), "\nfunc (f *Foo) SetClient(_ string) {}\n")

	_, err := RepositoryUnport(RepositoryUnportOptions{Root: root, Domain: "Foo"})
	if err == nil {
		t.Fatalf("esperado erro de estado parcial")
	}
	if !strings.Contains(err.Error(), "inconsistente") {
		t.Errorf("erro %q não menciona inconsistente", err.Error())
	}
}

func TestRepositoryUnport_FailsIfDomainFileMissing(t *testing.T) {
	root := t.TempDir()
	_, err := RepositoryUnport(RepositoryUnportOptions{Root: root, Domain: "Missing"})
	if err == nil {
		t.Fatalf("esperado erro pra domínio inexistente")
	}
}

func TestRepositoryUnport_InvalidDomainName(t *testing.T) {
	root := t.TempDir()
	_, err := RepositoryUnport(RepositoryUnportOptions{Root: root, Domain: "lowercase"})
	if err == nil {
		t.Fatalf("esperado erro pra nome inválido")
	}
}

func TestRepositoryUnport_RoundTrip(t *testing.T) {
	// port → unport → port → unport. Cada passo deve ter sucesso e a saída
	// final deve compilar no parser.
	root := t.TempDir()
	rel := seedDomain(t, root, "Foo")
	baseline := readFile(t, filepath.Join(root, rel))

	for i := 0; i < 2; i++ {
		if _, err := RepositoryPort(RepositoryPortOptions{Root: root, Domain: "Foo"}); err != nil {
			t.Fatalf("port[%d]: %v", i, err)
		}
		if _, err := RepositoryUnport(RepositoryUnportOptions{Root: root, Domain: "Foo"}); err != nil {
			t.Fatalf("unport[%d]: %v", i, err)
		}
	}

	final := readFile(t, filepath.Join(root, rel))
	if err := parseGoSrc([]byte(final)); err != nil {
		t.Fatalf("arquivo final não compila: %v\n%s", err, final)
	}
	// Não exigimos byte-equivalência (formatação pode diferir após round-trip),
	// mas a struct original deve ter sobrevivido.
	if !strings.Contains(final, "type Foo struct") {
		t.Errorf("struct Foo perdida após round-trip\n--- baseline ---\n%s\n--- final ---\n%s", baseline, final)
	}
	mustNotContain(t, final,
		"type Repository",
		"filters *filters.Filters",
		"func (f Foo) Schema()",
	)
}

func TestRepositoryUnport_RemovesInterfaceWithCustomMethods(t *testing.T) {
	// Caso `usage_record`: Repository com embed + métodos custom. A interface
	// inteira deve ser removida; warning é responsabilidade do CLI (testado em
	// repositoryHasCustomMethods).
	root := t.TempDir()
	rel := portFoo(t, root, "Foo", false)

	// Substitui o type Repository pelo shape com métodos custom.
	path := filepath.Join(root, rel)
	src := readFile(t, path)
	src = strings.Replace(src,
		"type Repository interface {\n\tbase_repository.BaseRepository[Foo]\n}",
		"type Repository interface {\n\tbase_repository.BaseRepository[Foo]\n\tUpsertBatch(ctx context.Context, items []Foo) error\n}",
		1)
	rewriteDomainFile(t, path, src)

	got := readFile(t, filepath.Join(root, rel))
	if err := parseGoSrc([]byte(got)); err != nil {
		// O seed não precisa compilar com context — só precisa parser-aceitar.
		t.Logf("seed parser warning: %v", err)
	}

	if !repositoryHasCustomMethods(parseOrFail(t, got)) {
		t.Fatalf("seed: helper repositoryHasCustomMethods deveria reportar true")
	}

	if _, err := RepositoryUnport(RepositoryUnportOptions{Root: root, Domain: "Foo"}); err != nil {
		t.Fatalf("RepositoryUnport: %v", err)
	}
	after := readFile(t, filepath.Join(root, rel))
	mustNotContain(t, after, "type Repository", "UpsertBatch")
}
