package runtimefs

import (
	"embed"
	"io/fs"
)

// embedded carrega o runtime materializado (ver ./sync). O prefixo `all:`
// inclui os dotfiles (.air.toml, .env.*, .gitignore, .dockerignore) e o próprio
// diretório `_runtime` (nome com prefixo `_`), que sem `all:` seriam excluídos.
//
//go:embed all:_runtime
var embedded embed.FS

// Runtime retorna o FS do runtime boilerplate com a raiz no conteúdo de
// _runtime/ — o `zord new` o percorre e materializa num projeto novo. O manifesto
// do módulo vem como `go.mod.embed` (evita módulo aninhado); o `zord new`
// o grava como `go.mod` no destino.
func Runtime() (fs.FS, error) {
	return fs.Sub(embedded, "_runtime")
}
