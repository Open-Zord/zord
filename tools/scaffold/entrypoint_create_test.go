package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEntrypointCreate_HappyPath_Simple(t *testing.T) {
	root := t.TempDir()
	writeFileTree(t, root, map[string]string{
		"go.mod": "module zord\n\ngo 1.22\n",
	})

	bp, cp, err := EntrypointCreate(EntrypointCreateOptions{Root: root, Name: "cli"})
	if err != nil {
		t.Fatalf("EntrypointCreate: %v", err)
	}
	if want := filepath.Join("bootstrap", "cli", "setup.go"); bp != want {
		t.Errorf("bootstrap path: got %q, want %q", bp, want)
	}
	if want := filepath.Join("cmd", "cli", "main.go"); cp != want {
		t.Errorf("cmd path: got %q, want %q", cp, want)
	}
	gotBoot := readFile(t, filepath.Join(root, bp))
	mustContain(t, gotBoot,
		"// Package cli wires the cli entrypoint dependencies into pkg/registry.",
		"package cli",
		`"zord/pkg/registry"`,
		"func Setup() *registry.Registry {",
		"reg := registry.NewRegistry()",
		"return reg",
	)
	gotCmd := readFile(t, filepath.Join(root, cp))
	mustContain(t, gotCmd,
		"// Command cli é o entrypoint cli",
		"package main",
		`cliboot "zord/bootstrap/cli"`,
		"func main() {",
		"reg := cliboot.Setup()",
		"_ = reg",
	)
	if err := parseGoSrc([]byte(gotBoot)); err != nil {
		t.Fatalf("setup.go gerado não compila no parser: %v\n%s", err, gotBoot)
	}
	if err := parseGoSrc([]byte(gotCmd)); err != nil {
		t.Fatalf("main.go gerado não compila no parser: %v\n%s", err, gotCmd)
	}
}

func TestEntrypointCreate_SnakeCaseName(t *testing.T) {
	root := t.TempDir()
	writeFileTree(t, root, map[string]string{
		"go.mod": "module zord\n\ngo 1.22\n",
	})

	bp, cp, err := EntrypointCreate(EntrypointCreateOptions{Root: root, Name: "redis_queue"})
	if err != nil {
		t.Fatalf("EntrypointCreate: %v", err)
	}
	gotBoot := readFile(t, filepath.Join(root, bp))
	mustContain(t, gotBoot,
		"package redis_queue",
		`"zord/pkg/registry"`,
	)
	gotCmd := readFile(t, filepath.Join(root, cp))
	// Alias do import remove underscores pra evitar nome de pacote feio
	// quando inline no main; mesmo padrão dos aliases de repository register.
	mustContain(t, gotCmd,
		`redisqueueboot "zord/bootstrap/redis_queue"`,
		"reg := redisqueueboot.Setup()",
	)
}

func TestEntrypointCreate_FailsIfBootstrapDirExists(t *testing.T) {
	root := t.TempDir()
	writeFileTree(t, root, map[string]string{
		"go.mod":                   "module zord\n\ngo 1.22\n",
		"bootstrap/grpc/setup.go": "package grpc\n",
	})
	_, _, err := EntrypointCreate(EntrypointCreateOptions{Root: root, Name: "grpc"})
	if err == nil {
		t.Fatal("esperava erro pra entrypoint pré-existente, got nil")
	}
	if !strings.Contains(err.Error(), "já existe") {
		t.Errorf("erro %q não menciona 'já existe'", err.Error())
	}
}

func TestEntrypointCreate_FailsIfCmdDirExists(t *testing.T) {
	root := t.TempDir()
	writeFileTree(t, root, map[string]string{
		"go.mod":              "module zord\n\ngo 1.22\n",
		"cmd/grpc/legacy.go": "package main\n",
	})
	_, _, err := EntrypointCreate(EntrypointCreateOptions{Root: root, Name: "grpc"})
	if err == nil {
		t.Fatal("esperava erro pra cmd/<name>/ pré-existente, got nil")
	}
	// Bootstrap não deveria ter sido criado (falha sem mutar disco).
	if _, statErr := os.Stat(filepath.Join(root, "bootstrap", "grpc")); !os.IsNotExist(statErr) {
		t.Fatalf("bootstrap/grpc/ foi criado apesar da falha em cmd/grpc/: %v", statErr)
	}
}

func TestEntrypointCreate_InvalidName(t *testing.T) {
	cases := []string{"", "HTTP", "Cli", "1grpc", "redis-queue", "func", "type"}
	for _, n := range cases {
		t.Run(n, func(t *testing.T) {
			root := t.TempDir()
			writeFileTree(t, root, map[string]string{"go.mod": "module zord\n\ngo 1.22\n"})
			_, _, err := EntrypointCreate(EntrypointCreateOptions{Root: root, Name: n})
			if err == nil {
				t.Errorf("EntrypointCreate(%q): esperava erro, got nil", n)
			}
		})
	}
}

func TestEntrypointCreate_DoesNotTouchExistingEntrypoints(t *testing.T) {
	root := t.TempDir()
	writeFileTree(t, root, map[string]string{
		"go.mod":                    "module zord\n\ngo 1.22\n",
		"bootstrap/http/setup.go":   "package http\n",
		"bootstrap/http/configs.go": "package http\n",
		"cmd/http/main.go":          "package main\n",
	})

	if _, _, err := EntrypointCreate(EntrypointCreateOptions{Root: root, Name: "cli"}); err != nil {
		t.Fatalf("EntrypointCreate: %v", err)
	}
	// Os arquivos do http precisam estar intactos.
	for _, path := range []string{
		"bootstrap/http/setup.go", "bootstrap/http/configs.go", "cmd/http/main.go",
	} {
		body := readFile(t, filepath.Join(root, path))
		if !strings.Contains(body, "package http") && !strings.Contains(body, "package main") {
			t.Errorf("arquivo %s foi tocado: %s", path, body)
		}
	}
}
