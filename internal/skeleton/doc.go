// Package skeleton embarca o runtime boilerplate do Zord (o subtree runtime/) e
// o expõe via Runtime(), de onde o comando `zord new` o materializa num projeto
// novo. O subtree é versionado e compilado como parte do módulo único — não há
// mais mirror gerado nem passo de `go generate`; a árvore commitada é a fonte do
// //go:embed, então `go install` funciona direto do fonte.
package skeleton
