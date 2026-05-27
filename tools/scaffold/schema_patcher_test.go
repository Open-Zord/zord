package scaffold

import (
	"strings"
	"testing"
)

func TestUnpatch_RemovesWrappedBlock(t *testing.T) {
	raw := []byte(`schema "zord" {}

# scaffold:generated widgets
table "widgets" {
  schema = schema.zord
}
# scaffold:end widgets
`)
	got, err := unpatch(raw, "widgets")
	if err != nil {
		t.Fatalf("unpatch: %v", err)
	}
	s := string(got)
	if strings.Contains(s, "scaffold:generated") || strings.Contains(s, "scaffold:end") {
		t.Errorf("sentinelas persistiram após unpatch:\n%s", s)
	}
	if strings.Contains(s, `table "widgets"`) {
		t.Errorf("table persistiu após unpatch:\n%s", s)
	}
	if !strings.HasPrefix(s, `schema "zord" {}`) {
		t.Errorf("conteúdo fora da sentinela alterado:\n%s", s)
	}
}

func TestUnpatch_PreservesNeighbors(t *testing.T) {
	raw := []byte(`# scaffold:generated alpha
table "alpha" {}
# scaffold:end alpha

# scaffold:generated widgets
table "widgets" {}
# scaffold:end widgets

# scaffold:generated beta
table "beta" {}
# scaffold:end beta
`)
	got, err := unpatch(raw, "widgets")
	if err != nil {
		t.Fatalf("unpatch: %v", err)
	}
	s := string(got)
	for _, want := range []string{
		"# scaffold:generated alpha",
		"# scaffold:end alpha",
		`table "alpha"`,
		"# scaffold:generated beta",
		"# scaffold:end beta",
		`table "beta"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("vizinho %q removido indevidamente:\n%s", want, s)
		}
	}
	if strings.Contains(s, "widgets") {
		t.Errorf("widgets persistiu:\n%s", s)
	}
}

func TestUnpatch_FailsOnMissingSentinel(t *testing.T) {
	raw := []byte(`schema "zord" {}
`)
	_, err := unpatch(raw, "widgets")
	if err == nil {
		t.Fatal("esperava erro de sentinela ausente; got nil")
	}
	if !strings.Contains(err.Error(), "sentinela ausente") {
		t.Errorf("erro inesperado: %v", err)
	}
}

func TestUnpatch_FailsOnPartialSentinel(t *testing.T) {
	raw := []byte(`# scaffold:generated widgets
table "widgets" {}
`)
	_, err := unpatch(raw, "widgets")
	if err == nil {
		t.Fatal("esperava erro de sentinela parcial; got nil")
	}
	if !strings.Contains(err.Error(), "sem :end") {
		t.Errorf("erro inesperado: %v", err)
	}
}
