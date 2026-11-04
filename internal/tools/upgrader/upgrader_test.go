package upgrader

import (
	"errors"
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareRelease(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v1.0.0", "v1.1.0", -1},
		{"v1.2.0", "v1.1.9", 1},
		{"v1.0.0", "v2.0.0", -1},
		{"v1.0.10", "v1.0.9", 1},
	}
	for _, c := range cases {
		if got := compareRelease(c.a, c.b); got != c.want {
			t.Errorf("compareRelease(%q,%q) = %d; want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestIsRelease(t *testing.T) {
	for v, want := range map[string]bool{
		"v1.0.0": true, "1.2.3": true, "v1.2": false, "dev": false,
		"v1.0.1-0.2026-abc": false, "": false,
	} {
		if got := isRelease(v); got != want {
			t.Errorf("isRelease(%q) = %v; want %v", v, got, want)
		}
	}
}

func migs(versions ...string) []Migration {
	out := make([]Migration, len(versions))
	for i, v := range versions {
		out[i] = Migration{Version: v, Description: "m " + v, Apply: func(_, _ string) error { return nil }}
	}
	return out
}

func versionsOf(ms []Migration) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Version
	}
	return out
}

func TestPending(t *testing.T) {
	all := migs("v1.3.0", "v1.1.0", "v1.2.0") // fora de ordem de propósito

	// from=v1.0.0, target=v1.2.0 → v1.1.0, v1.2.0 (ordenadas)
	got := versionsOf(Pending("v1.0.0", "v1.2.0", all))
	if strings.Join(got, ",") != "v1.1.0,v1.2.0" {
		t.Fatalf("Pending normal = %v", got)
	}

	// já na última → nada
	if got := Pending("v1.3.0", "v1.3.0", all); len(got) != 0 {
		t.Fatalf("esperava vazio no topo, veio %v", versionsOf(got))
	}

	// projeto sem carimbo (baseline) + CLI dev (∞) → todas
	got = versionsOf(Pending("", "dev", all))
	if strings.Join(got, ",") != "v1.1.0,v1.2.0,v1.3.0" {
		t.Fatalf("Pending baseline+dev = %v", got)
	}
}

func TestApplyPending_OrderAndStop(t *testing.T) {
	var order []string
	ok := func(v string) Migration {
		return Migration{Version: v, Apply: func(_, _ string) error { order = append(order, v); return nil }}
	}
	boom := Migration{Version: "v1.2.0", Apply: func(_, _ string) error { return errors.New("boom") }}

	applied, err := ApplyPending("", "", "v1.0.0", "v2.0.0", []Migration{ok("v1.1.0"), boom, ok("v1.3.0")})
	if err == nil {
		t.Fatal("esperava erro da migration boom")
	}
	if strings.Join(versionsOf(applied), ",") != "v1.1.0" {
		t.Fatalf("deveria ter aplicado só v1.1.0 antes do erro, veio %v", versionsOf(applied))
	}
	if strings.Join(order, ",") != "v1.1.0" {
		t.Fatalf("ordem de execução inesperada: %v", order)
	}
}

func TestVersionRoundTrip(t *testing.T) {
	root := t.TempDir()

	if v, err := ReadVersion(root); err != nil || v != "" {
		t.Fatalf("ausente deveria dar (\"\", nil); veio (%q, %v)", v, err)
	}
	if err := WriteVersion(root, "v1.2.0"); err != nil {
		t.Fatalf("WriteVersion: %v", err)
	}
	if v, _ := ReadVersion(root); v != "v1.2.0" {
		t.Fatalf("round-trip = %q; want v1.2.0", v)
	}
}

// TestApplyPending_RealCodemod prova o pipeline fim-a-fim: uma Migration cujo
// Apply é um codemod AST de verdade (renomeia o identificador Foo→Bar via
// WalkGoFiles+RewriteGoFile) é aplicada por ApplyPending e reescreve o arquivo.
func TestApplyPending_RealCodemod(t *testing.T) {
	root := t.TempDir()
	src := "package demo\n\nfunc Foo() int { return 1 }\n\nfunc use() int { return Foo() }\n"
	if err := os.WriteFile(filepath.Join(root, "x.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	renameFooToBar := Migration{
		Version: "v1.1.0",
		Apply: func(root, _ string) error {
			return WalkGoFiles(root, func(path string) error {
				return RewriteGoFile(path, func(file *ast.File) (bool, error) {
					changed := false
					ast.Inspect(file, func(n ast.Node) bool {
						if id, ok := n.(*ast.Ident); ok && id.Name == "Foo" {
							id.Name = "Bar"
							changed = true
						}
						return true
					})
					return changed, nil
				})
			})
		},
	}

	applied, err := ApplyPending(root, "example.com/demo", "v1.0.0", "v1.1.0", []Migration{renameFooToBar})
	if err != nil {
		t.Fatalf("ApplyPending: %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("esperava 1 migration aplicada, veio %d", len(applied))
	}

	out, _ := os.ReadFile(filepath.Join(root, "x.go"))
	got := string(out)
	if strings.Contains(got, "Foo") || !strings.Contains(got, "func Bar()") || !strings.Contains(got, "return Bar()") {
		t.Fatalf("codemod não reescreveu Foo→Bar:\n%s", got)
	}
}
