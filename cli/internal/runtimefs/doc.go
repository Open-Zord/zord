// Package runtimefs materializa e (na Fase 1) embarca o runtime subtree do
// monorepo, que é o boilerplate emitido por `zord new`.
//
// Fase 0 define apenas o mecanismo de materialização (comando ./sync): ele
// copia o subtree da raiz para _runtime/. A diretiva //go:embed e o acesso ao
// embed.FS entram na Fase 1, apontando para _runtime via `//go:embed all:_runtime`.
//
//go:generate go run ./sync
package runtimefs
