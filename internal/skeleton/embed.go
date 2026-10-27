package skeleton

import (
	"embed"
	"io/fs"
)

// embedded carrega o runtime boilerplate — o subtree versionado em runtime/, que
// o `zord new` materializa num projeto novo. O prefixo `all:` inclui os dotfiles
// (.air.toml, .env.*, .gitignore, .dockerignore) que sem ele seriam excluídos do
// embed. runtime/ tem `go.mod.embed` (não `go.mod`), então o //go:embed não vê
// fronteira de módulo e o atravessa; os .go compilam sob o módulo único no path
// github.com/Open-Zord/zord/internal/skeleton/runtime/..., preservando o dogfood.
//
//go:embed all:runtime
var embedded embed.FS

// Runtime retorna o FS do runtime boilerplate com a raiz no conteúdo de runtime/
// — o `zord new` o percorre e materializa num projeto novo. O manifesto do módulo
// vem como `go.mod.embed` (evita módulo aninhado); o `zord new` o grava como
// `go.mod` no destino.
func Runtime() (fs.FS, error) {
	return fs.Sub(embedded, "runtime")
}
