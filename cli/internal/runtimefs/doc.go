// Package runtimefs materializa e embarca o runtime subtree do monorepo, que é
// o boilerplate emitido por `zord new`.
//
// O comando ./sync (go generate) copia o subtree da raiz para _runtime/, e
// embed.go o embarca via //go:embed, expondo-o por Runtime().
//
// _runtime/ é um artefato GERADO e gitignored — não é cópia versionada no repo,
// a arquitetura do runtime vive só na raiz. Consequência: num clone limpo é
// preciso rodar `make embed-sync` (ou `go generate ./...`) antes de compilar o
// cli, senão o //go:embed falha por falta de arquivos. A forma de empacotar o
// embed para `go install` (artefato commitado vs. binário pré-compilado) fica
// pra uma fase posterior de distribuição.
//
//go:generate go run ./sync
package runtimefs
