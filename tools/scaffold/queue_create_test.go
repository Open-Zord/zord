package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueueCreate_HappyPath_Simple(t *testing.T) {
	root := t.TempDir()
	writeFileTree(t, root, map[string]string{
		"go.mod": "module zord\n\ngo 1.22\n",
	})

	files, err := QueueCreate(QueueCreateOptions{Root: root, Name: "worker"})
	if err != nil {
		t.Fatalf("QueueCreate: %v", err)
	}

	wantPaths := []string{
		filepath.Join("bootstrap", "worker", "setup.go"),
		filepath.Join("bootstrap", "worker", "configs.go"),
		filepath.Join("bootstrap", "worker", "pkg.go"),
		filepath.Join("bootstrap", "worker", "repositories.go"),
		filepath.Join("bootstrap", "worker", "services.go"),
		filepath.Join("bootstrap", "worker", "worker.go"),
		filepath.Join("cmd", "worker", "main.go"),
	}
	if len(files) != len(wantPaths) {
		t.Fatalf("paths count: got %d %v, want %d %v", len(files), files, len(wantPaths), wantPaths)
	}
	for i, want := range wantPaths {
		if files[i] != want {
			t.Errorf("path[%d]: got %q, want %q", i, files[i], want)
		}
	}

	// setup.go: package + Setup + Run + omniq import + queueName from configs
	setup := readFile(t, filepath.Join(root, "bootstrap", "worker", "setup.go"))
	mustContain(t, setup,
		"package worker",
		`"zord/pkg/registry"`,
		`"github.com/not-empty/omniq-go"`,
		`const omniqClientRegistryKey = "omniq.client"`,
		"func Setup() (reg *registry.Registry, queueName string)",
		"func Run() error",
		"client := registry.Resolve[*omniq.Client](reg, omniqClientRegistryKey)",
		"Handler: NewHandler(reg)",
	)

	// configs.go: loadConfigs + OMNIQ_QUEUE
	configs := readFile(t, filepath.Join(root, "bootstrap", "worker", "configs.go"))
	mustContain(t, configs,
		"package worker",
		`"zord/pkg/config"`,
		"func loadConfigs() (conf *config.Config, queueName string)",
		`conf.ReadConfig("OMNIQ_QUEUE")`,
	)

	// pkg.go: registerPkg + omniq client construction
	pkg := readFile(t, filepath.Join(root, "bootstrap", "worker", "pkg.go"))
	mustContain(t, pkg,
		"package worker",
		`"github.com/not-empty/omniq-go"`,
		"func registerPkg(reg *registry.Registry, conf *config.Config)",
		"client, err := omniq.NewClient(omniq.ClientOpts{",
		`conf.ReadConfig("OMNIQ_HOST")`,
		`conf.ReadNumberConfig("OMNIQ_PORT")`,
		`conf.ReadNumberConfig("OMNIQ_DB")`,
		"reg.Provide(omniqClientRegistryKey, client)",
	)

	// repositories.go: empty body, registry import only
	repos := readFile(t, filepath.Join(root, "bootstrap", "worker", "repositories.go"))
	mustContain(t, repos,
		"package worker",
		`"zord/pkg/registry"`,
		"func registerRepositories(reg *registry.Registry) {",
	)

	// services.go: empty body
	services := readFile(t, filepath.Join(root, "bootstrap", "worker", "services.go"))
	mustContain(t, services,
		"package worker",
		`"zord/pkg/registry"`,
		"func registerServices(reg *registry.Registry) {",
	)

	// worker.go: NewHandler stub
	worker := readFile(t, filepath.Join(root, "bootstrap", "worker", "worker.go"))
	mustContain(t, worker,
		"package worker",
		`"github.com/not-empty/omniq-go"`,
		"func NewHandler(reg *registry.Registry) omniq.ConsumeHandler",
		"return func(ctx omniq.JobCtx)",
	)

	// cmd/worker/main.go: workerboot.Run() one-liner
	main := readFile(t, filepath.Join(root, "cmd", "worker", "main.go"))
	mustContain(t, main,
		"package main",
		`workerboot "zord/bootstrap/worker"`,
		"workerboot.Run()",
		`log.Fatalf("queue worker: %v", err)`,
	)

	// All files must parse.
	for _, rel := range wantPaths {
		body := readFile(t, filepath.Join(root, rel))
		if err := parseGoSrc([]byte(body)); err != nil {
			t.Fatalf("arquivo %s não compila no parser: %v\n%s", rel, err, body)
		}
	}
}

func TestQueueCreate_SnakeCaseName(t *testing.T) {
	root := t.TempDir()
	writeFileTree(t, root, map[string]string{
		"go.mod": "module zord\n\ngo 1.22\n",
	})

	files, err := QueueCreate(QueueCreateOptions{Root: root, Name: "billing_worker"})
	if err != nil {
		t.Fatalf("QueueCreate: %v", err)
	}
	if len(files) != 7 {
		t.Fatalf("paths count: got %d %v", len(files), files)
	}

	setup := readFile(t, filepath.Join(root, "bootstrap", "billing_worker", "setup.go"))
	mustContain(t, setup, "package billing_worker")

	// Alias do import remove underscores pra evitar nome de pacote feio
	// quando inline no main; mesmo padrão dos aliases de repository register.
	main := readFile(t, filepath.Join(root, "cmd", "billing_worker", "main.go"))
	mustContain(t, main,
		`billingworkerboot "zord/bootstrap/billing_worker"`,
		"billingworkerboot.Run()",
	)
}

func TestQueueCreate_DefaultDriverIsOmniq(t *testing.T) {
	root := t.TempDir()
	writeFileTree(t, root, map[string]string{
		"go.mod": "module zord\n\ngo 1.22\n",
	})

	// Driver vazio == omniq. Não deve falhar.
	if _, err := QueueCreate(QueueCreateOptions{Root: root, Name: "worker"}); err != nil {
		t.Fatalf("QueueCreate sem Driver: %v", err)
	}
	pkg := readFile(t, filepath.Join(root, "bootstrap", "worker", "pkg.go"))
	if !strings.Contains(pkg, "omniq.NewClient") {
		t.Errorf("driver default deveria ser omniq, mas pkg.go não tem omniq.NewClient")
	}
}

func TestQueueCreate_UnsupportedDriver(t *testing.T) {
	root := t.TempDir()
	writeFileTree(t, root, map[string]string{"go.mod": "module zord\n\ngo 1.22\n"})

	_, err := QueueCreate(QueueCreateOptions{Root: root, Name: "worker", Driver: "sqs"})
	if err == nil {
		t.Fatal("esperava erro pra driver não suportado, got nil")
	}
	if !strings.Contains(err.Error(), "driver de fila não suportado") {
		t.Errorf("erro %q não menciona driver não suportado", err.Error())
	}
	// Nenhum diretório deve ter sido criado.
	if _, statErr := os.Stat(filepath.Join(root, "bootstrap", "worker")); !os.IsNotExist(statErr) {
		t.Fatalf("bootstrap/worker/ criado apesar de driver inválido: %v", statErr)
	}
}

func TestQueueCreate_FailsIfBootstrapDirExists(t *testing.T) {
	root := t.TempDir()
	writeFileTree(t, root, map[string]string{
		"go.mod":                     "module zord\n\ngo 1.22\n",
		"bootstrap/worker/setup.go":  "package worker\n",
	})
	_, err := QueueCreate(QueueCreateOptions{Root: root, Name: "worker"})
	if err == nil {
		t.Fatal("esperava erro pra entrypoint pré-existente, got nil")
	}
	if !strings.Contains(err.Error(), "já existe") {
		t.Errorf("erro %q não menciona 'já existe'", err.Error())
	}
}

func TestQueueCreate_FailsIfCmdDirExists(t *testing.T) {
	root := t.TempDir()
	writeFileTree(t, root, map[string]string{
		"go.mod":              "module zord\n\ngo 1.22\n",
		"cmd/worker/legacy.go": "package main\n",
	})
	_, err := QueueCreate(QueueCreateOptions{Root: root, Name: "worker"})
	if err == nil {
		t.Fatal("esperava erro pra cmd/<name>/ pré-existente, got nil")
	}
	// Bootstrap não deveria ter sido criado (falha sem mutar disco).
	if _, statErr := os.Stat(filepath.Join(root, "bootstrap", "worker")); !os.IsNotExist(statErr) {
		t.Fatalf("bootstrap/worker/ foi criado apesar da falha em cmd/worker/: %v", statErr)
	}
}

func TestQueueCreate_InvalidName(t *testing.T) {
	cases := []string{"", "Worker", "WORKER", "1worker", "billing-worker", "func", "type"}
	for _, n := range cases {
		t.Run(n, func(t *testing.T) {
			root := t.TempDir()
			writeFileTree(t, root, map[string]string{"go.mod": "module zord\n\ngo 1.22\n"})
			_, err := QueueCreate(QueueCreateOptions{Root: root, Name: n})
			if err == nil {
				t.Errorf("QueueCreate(%q): esperava erro, got nil", n)
			}
		})
	}
}

func TestQueueCreate_DoesNotTouchExistingEntrypoints(t *testing.T) {
	root := t.TempDir()
	writeFileTree(t, root, map[string]string{
		"go.mod":                    "module zord\n\ngo 1.22\n",
		"bootstrap/http/setup.go":   "package http\n",
		"bootstrap/http/configs.go": "package http\n",
		"cmd/http/main.go":          "package main\n",
	})

	if _, err := QueueCreate(QueueCreateOptions{Root: root, Name: "worker"}); err != nil {
		t.Fatalf("QueueCreate: %v", err)
	}
	for _, path := range []string{
		"bootstrap/http/setup.go", "bootstrap/http/configs.go", "cmd/http/main.go",
	} {
		body := readFile(t, filepath.Join(root, path))
		if !strings.Contains(body, "package http") && !strings.Contains(body, "package main") {
			t.Errorf("arquivo %s foi tocado: %s", path, body)
		}
	}
}
