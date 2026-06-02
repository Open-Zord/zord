package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// RegistryKey is the key under which the config is registered in the registry.
const RegistryKey = "config"

type Config struct {
}

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) ReadConfig(Key string) string {
	env := os.Getenv(Key)
	if env == "" {
		panic(fmt.Sprintf("[Config] Unable to read environment variable: %v", Key))
	}
	return env
}

func (c *Config) ReadNumberConfig(Key string) int {
	env := os.Getenv(Key)
	if env == "" {
		panic(fmt.Sprintf("[Config] Unable to read environment variable: %v", Key))
	}
	config, err := strconv.Atoi(env)
	if err != nil {
		panic(fmt.Sprintf("[Config] Jwt Expiration (minutes) must be integer: %v", Key))
	}
	return config
}

func (c *Config) ReadArrayConfig(Key string) []string {
	env := os.Getenv(Key)
	if env == "" {
		panic(fmt.Sprintf("[Config] Unable to read environment variable: %v", Key))
	}
	config := strings.Split(env, ",")
	return config
}

// LoadEnvsForEntrypoint carrega envs do disco em uma ordem de prioridade que
// dá conta dos três cenários esperados:
//
//  1. `.env` presente (gitignored, creds reais de homolog/staging) — wins
//     sobre `.env.<entrypoint>` e é carregado em exclusivo.
//  2. `.env.<entrypoint>` presente (versionado, defaults de dev local)
//     — carregado quando não há `.env`.
//  3. Nenhum dos dois — no-op; o binário lê apenas envs do sistema (caso
//     padrão de produção, onde o orquestrador injeta valores).
//
// Em todos os cenários, `godotenv.Load` (não `Overload`) só seta envs que
// ainda NÃO estão definidos no processo, então valores injetados pelo
// sistema sempre vencem sobre o conteúdo de arquivo. Retorna o erro do
// `godotenv.Load` quando o arquivo escolhido existe mas falha em parsear;
// arquivo ausente em todos os níveis NÃO é erro (system envs cobrem).
//
// `entrypoint` vazio pula o passo 2 — útil pra ferramentas standalone que
// não conhecem um entrypoint específico.
func (c *Config) LoadEnvsForEntrypoint(entrypoint string) error {
	if _, err := os.Stat(".env"); err == nil {
		return godotenv.Load(".env")
	}
	if entrypoint != "" {
		per := ".env." + entrypoint
		if _, err := os.Stat(per); err == nil {
			return godotenv.Load(per)
		}
	}
	return nil
}
