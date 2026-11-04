package main

import "testing"

func TestResolveVersion_LdflagsWins(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	version = "v1.2.3"
	if got := resolveVersion(); got != "v1.2.3" {
		t.Fatalf("esperava a versão de ldflags, veio %q", got)
	}
}

func TestResolveVersion_FallbackDev(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	// Sem ldflags, num binário de teste o BuildInfo.Main.Version é "" ou
	// "(devel)" → cai em "dev".
	version = ""
	if got := resolveVersion(); got != "dev" {
		t.Fatalf("esperava fallback \"dev\", veio %q", got)
	}
}
