// Package runtimefs materializa e embarca o runtime subtree do monorepo, que é
// o boilerplate emitido por `zord new`.
//
// O comando ./sync (go generate) copia o subtree da raiz para _runtime/ (ver
// sync/main.go); embed.go o embarca via //go:embed e expõe via Runtime(). O
// _runtime/ é artefato gerado-commitado: o drift-guard (make embed-check) garante
// que ele não diverge da raiz, e commitá-lo é o que permite `go install` compilar
// o embed a partir do module zip.
//
//go:generate go run ./sync
package runtimefs
