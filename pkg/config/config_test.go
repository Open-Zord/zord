package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfig(t *testing.T) {
	c := NewConfig()
	assert.NotNil(t, c)
}

func TestReadConfig(t *testing.T) {
	key := "TEST_KEY"
	value := "test_value"
	os.Setenv(key, value)
	defer os.Unsetenv(key)

	c := NewConfig()
	result := c.ReadConfig(key)

	assert.Equal(t, value, result)
}

func TestReadNumberConfig(t *testing.T) {
	key := "TEST_NUMBER_KEY"
	value := "42"
	os.Setenv(key, value)
	defer os.Unsetenv(key)

	c := NewConfig()
	result := c.ReadNumberConfig(key)

	assert.Equal(t, 42, result)
}

func TestReadArrayConfig(t *testing.T) {
	key := "TEST_ARRAY_KEY"
	value := "value1,value2,value3"
	os.Setenv(key, value)
	defer os.Unsetenv(key)

	c := NewConfig()
	result := c.ReadArrayConfig(key)

	assert.Equal(t, []string{"value1", "value2", "value3"}, result)
}

// TestLoadEnvsForEntrypoint_DotEnvWins valida prioridade 1: quando `.env`
// existe, é carregado em exclusivo, ignorando `.env.<entrypoint>` mesmo que
// presente. Casa com o caso "homolog override de creds reais".
func TestLoadEnvsForEntrypoint_DotEnvWins(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".env"), "TEST_LOADENVS_KEY=fromDotEnv\n")
	writeFile(t, filepath.Join(root, ".env.worker"), "TEST_LOADENVS_KEY=fromDotEnvWorker\n")
	defer os.Unsetenv("TEST_LOADENVS_KEY")

	withCwd(t, root, func() {
		c := NewConfig()
		err := c.LoadEnvsForEntrypoint("worker")
		assert.NoError(t, err)
		assert.Equal(t, "fromDotEnv", os.Getenv("TEST_LOADENVS_KEY"))
	})
}

// TestLoadEnvsForEntrypoint_PerEntrypointFallback valida prioridade 2: na
// ausência de `.env`, o `.env.<entrypoint>` é carregado. Casa com o caso
// "dev local rodando com defaults versionados".
func TestLoadEnvsForEntrypoint_PerEntrypointFallback(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".env.worker"), "TEST_LOADENVS_KEY=fromDotEnvWorker\n")
	defer os.Unsetenv("TEST_LOADENVS_KEY")

	withCwd(t, root, func() {
		c := NewConfig()
		err := c.LoadEnvsForEntrypoint("worker")
		assert.NoError(t, err)
		assert.Equal(t, "fromDotEnvWorker", os.Getenv("TEST_LOADENVS_KEY"))
	})
}

// TestLoadEnvsForEntrypoint_NoFilesNoError valida prioridade 3: sem nenhum
// arquivo, retorna nil (envs vêm exclusivamente do sistema). Garante que
// produção sem .env não quebra.
func TestLoadEnvsForEntrypoint_NoFilesNoError(t *testing.T) {
	root := t.TempDir()
	withCwd(t, root, func() {
		c := NewConfig()
		err := c.LoadEnvsForEntrypoint("worker")
		assert.NoError(t, err)
	})
}

// TestLoadEnvsForEntrypoint_EmptyName pula o passo 2 e tenta só `.env`.
// Útil pra ferramentas standalone sem entrypoint definido.
func TestLoadEnvsForEntrypoint_EmptyName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".env.worker"), "TEST_LOADENVS_KEY=fromDotEnvWorker\n")
	defer os.Unsetenv("TEST_LOADENVS_KEY")

	withCwd(t, root, func() {
		c := NewConfig()
		err := c.LoadEnvsForEntrypoint("")
		assert.NoError(t, err)
		assert.Equal(t, "", os.Getenv("TEST_LOADENVS_KEY"))
	})
}

// TestLoadEnvsForEntrypoint_SystemEnvWins confirma que `godotenv.Load` (não
// `Overload`) preserva envs já injetados no processo — exatamente o que prod
// precisa: arquivo serve só de fallback, sistema manda.
func TestLoadEnvsForEntrypoint_SystemEnvWins(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".env"), "TEST_LOADENVS_KEY=fromDotEnv\n")
	os.Setenv("TEST_LOADENVS_KEY", "fromSystem")
	defer os.Unsetenv("TEST_LOADENVS_KEY")

	withCwd(t, root, func() {
		c := NewConfig()
		err := c.LoadEnvsForEntrypoint("worker")
		assert.NoError(t, err)
		assert.Equal(t, "fromSystem", os.Getenv("TEST_LOADENVS_KEY"))
	})
}

// withCwd executa fn com o diretório de trabalho temporariamente trocado pra
// dir; restaura o original mesmo se fn panicar. Necessário porque
// godotenv.Load resolve paths relativos contra o cwd do processo.
func withCwd(t *testing.T, dir string, fn func()) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("chdir back to %s: %v", orig, err)
		}
	}()
	fn()
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
