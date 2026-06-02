// Package scaffold (área entrypoint) cria o esqueleto mínimo de um novo
// entrypoint do monorepo: um arquivo por camada (cmd/<name>/main.go e
// bootstrap/<name>/setup.go), com `Setup()` retornando um *registry.Registry
// vazio. É a base sobre a qual scaffolds específicos de tipo (grpc, graphql,
// k8s operator, redis_queue, etc.) virão depois — cada um trazendo seus
// próprios arquivos por camada.
package scaffold

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
)

// cmdBasePath é o diretório que agrupa todos os entrypoints (cmd/<name>/).
// bootstrapBasePath vive em register_service.go porque os comandos de register
// precisaram dele primeiro.
const cmdBasePath = "cmd"

// EntrypointCreateOptions parametriza EntrypointCreate.
type EntrypointCreateOptions struct {
	// Root é a raiz do repositório. Vazio usa o diretório de trabalho atual.
	Root string
	// Name é o nome do entrypoint em lowercase (ex.: "cli", "grpc",
	// "redis_queue"). Vira o nome do pacote Go em bootstrap/<name>/ e o
	// segmento de diretório em cmd/<name>/.
	Name string
}

// EntrypointCreate gera os arquivos mínimos do novo entrypoint:
//
//   - bootstrap/<name>/setup.go com `package <name>` e `func Setup() *registry.Registry`
//     que apenas instancia o registry — extensão incremental fica a cargo de
//     scaffolds futuros específicos por tipo (grpc, queue, etc.).
//   - cmd/<name>/main.go com `package main` que chama `<name>boot.Setup()` e
//     descarta o registry com `_ = reg`; é esqueleto que compila e dá o
//     gancho pro dev inserir o runtime do entrypoint.
//
// Validações (todas obrigatórias, falham sem mutar disco):
//   - Name é identificador Go válido em lowercase (não keyword).
//   - Nenhum dos diretórios alvo (cmd/<name>/ ou bootstrap/<name>/) existe.
//
// Retorna os dois caminhos relativos editados, na ordem (bootstrap, cmd).
func EntrypointCreate(opts EntrypointCreateOptions) (bootstrapPath, cmdPath string, err error) {
	if !IsValidPackageIdent(opts.Name) {
		return "", "", fmt.Errorf("nome de entrypoint inválido (esperado lowercase Go ident, não keyword): %q", opts.Name)
	}

	root := opts.Root
	if root == "" {
		root = "."
	}

	relBootstrapDir := filepath.Join(bootstrapBasePath, opts.Name)
	relBootstrapFile := filepath.Join(relBootstrapDir, "setup.go")
	absBootstrapDir := filepath.Join(root, relBootstrapDir)
	absBootstrapFile := filepath.Join(root, relBootstrapFile)

	relCmdDir := filepath.Join(cmdBasePath, opts.Name)
	relCmdFile := filepath.Join(relCmdDir, "main.go")
	absCmdDir := filepath.Join(root, relCmdDir)
	absCmdFile := filepath.Join(root, relCmdFile)

	// Pré-checagens em sequência pra falhar sem mutar disco. Diretório
	// existente é o gate único — cobre tanto o caso "entrypoint já criado"
	// quanto "colisão com pasta legada que ninguém apagou".
	if err := assertDirAbsent(absBootstrapDir, relBootstrapDir); err != nil {
		return "", "", err
	}
	if err := assertDirAbsent(absCmdDir, relCmdDir); err != nil {
		return "", "", err
	}

	imp, err := newImportPaths(root)
	if err != nil {
		return "", "", err
	}

	bootstrapSrc, err := buildEntrypointSetupFile(opts.Name, imp)
	if err != nil {
		return "", "", err
	}
	cmdSrc, err := buildEntrypointMainFile(opts.Name, imp)
	if err != nil {
		return "", "", err
	}

	// Ordem de mutação: bootstrap primeiro, cmd depois. Se a escrita do cmd
	// falhar, o bootstrap fica órfão — mas isso é tratado como erro reportado
	// (não rollback), seguindo a postura dos demais comandos do scaffold (o
	// dev resolve manualmente). Rollback parcial introduziria janelas onde
	// o filesystem pode estar inconsistente em outros eixos (lock, perm).
	if err := writeNewFile(absBootstrapDir, absBootstrapFile, bootstrapSrc); err != nil {
		return "", "", err
	}
	if err := writeNewFile(absCmdDir, absCmdFile, cmdSrc); err != nil {
		return "", "", err
	}
	return relBootstrapFile, relCmdFile, nil
}

func assertDirAbsent(absDir, relDir string) error {
	if _, err := os.Stat(absDir); err == nil {
		return fmt.Errorf("entrypoint já existe: %s", relDir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", absDir, err)
	}
	return nil
}

func writeNewFile(absDir, absFile string, src []byte) error {
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", absDir, err)
	}
	if err := os.WriteFile(absFile, src, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", absFile, err)
	}
	return nil
}

// buildEntrypointSetupFile monta, via AST puro, o arquivo
// `bootstrap/<name>/setup.go`:
//
//	// Package <name> wires the <name> entrypoint dependencies into pkg/registry.
//	// Other entrypoints live in sibling subpackages of bootstrap/.
//	package <name>
//
//	import (
//	    "github.com/Open-Zord/zord/pkg/registry"
//	)
//
//	// Setup builds the dependency graph for the <name> entrypoint and returns
//	// the ready-to-use registry. Extend by registering primitives, repositories,
//	// services and adapters as the entrypoint takes shape.
//	func Setup() *registry.Registry {
//	    reg := registry.NewRegistry()
//	    return reg
//	}
func buildEntrypointSetupFile(name string, imp importPaths) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-entrypoint-setup")

	imports := ImportGroups(padder, []string{imp.join(registryImportSubpath)})

	// reg := registry.NewRegistry()
	regLhs := Ident("reg")
	regAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{regLhs},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{Fun: Sel("registry", "NewRegistry")}},
	}
	returnReg := ReturnStmt(Ident("reg"))

	setupDecl := FuncDecl(
		nil,
		"Setup",
		nil,
		FieldList(AnonField(StarOf(Sel("registry", "Registry")))),
		[]ast.Stmt{regAssign, returnReg},
	)
	setupDecl.Doc = singleComment(fmt.Sprintf(
		"// Setup builds the dependency graph for the %s entrypoint and returns the\n"+
			"// ready-to-use registry. Extend by registering primitives, repositories,\n"+
			"// services and adapters as the entrypoint takes shape.", name))

	packageDoc := singleComment(fmt.Sprintf(
		"// Package %s wires the %s entrypoint dependencies into pkg/registry.\n"+
			"// Other entrypoints live in sibling subpackages of bootstrap/.\n"+
			"//\n"+
			"//zord:entrypoint skeleton", name, name))

	// Posicionamento manual: pkg doc, package, imports já posicionados pelo
	// padder via ImportGroups, gap, depois Setup.
	packageDoc.List[0].Slash = padder.Take()
	packagePos := padder.Take()
	padder.Gap(1)
	stampDocPositions(padder, setupDecl.Doc)
	setupDecl.Type.Func = padder.Take()
	setupDecl.Body.Lbrace = padder.Take()
	regLhs.NamePos = padder.Take()
	regAssign.TokPos = regLhs.NamePos
	returnReg.Return = padder.Take()
	setupDecl.Body.Rbrace = padder.Take()

	file := &ast.File{
		Doc:     packageDoc,
		Package: packagePos,
		Name:    Ident(name),
		Decls:   []ast.Decl{imports, setupDecl},
	}
	file.Comments = collectDocs(packageDoc, file.Decls)

	return finalizeASTSource(fset, file)
}

// buildEntrypointMainFile monta, via AST puro, o arquivo `cmd/<name>/main.go`:
//
//	// Command <name> é o entrypoint <name> (esqueleto gerado por scaffold entrypoint create).
//	package main
//
//	import (
//	    <name>boot "<module>/bootstrap/<name>"
//	)
//
//	func main() {
//	    reg := <name>boot.Setup()
//	    _ = reg
//	}
//
// `_ = reg` é proposital: o registry sai do Setup pronto mas o runtime do
// entrypoint ainda não existe; sem o descarte explícito, o compilador
// reclamaria. Quem evoluir o entrypoint substitui essa linha pelo runtime
// real (servidor, worker, comando CLI, etc.).
func buildEntrypointMainFile(name string, imp importPaths) ([]byte, error) {
	fset := token.NewFileSet()
	padder := NewLinePadder(fset, "scaffold-entrypoint-main")

	bootAlias := stripUnderscores(name) + "boot"
	bootImportPath := imp.join(bootstrapBasePath + "/" + name)

	bootSpec := ImportSpec(bootImportPath)
	bootSpec.Name = Ident(bootAlias)
	bootSpec.Path.ValuePos = padder.Take()
	lparen := padder.Take()
	rparen := padder.Take()
	bootSpec.Name.NamePos = bootSpec.Path.ValuePos
	imports := &ast.GenDecl{
		Tok:    token.IMPORT,
		Lparen: lparen,
		Rparen: rparen,
		Specs:  []ast.Spec{bootSpec},
	}
	// Padder usado acima já consumiu três linhas pro import block; o
	// LinePadder começou em 1, então as próximas Take()s saem em sequência
	// natural. Pra forçar o paren-block multi-line o printer respeita as
	// Pos quando Lparen/Rparen/Path têm linhas distintas no FileSet.

	// reg := <alias>.Setup()
	regLhs := Ident("reg")
	regAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{regLhs},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{Fun: Sel(bootAlias, "Setup")}},
	}
	discardAssign := &ast.AssignStmt{
		Lhs: []ast.Expr{Ident("_")},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{Ident("reg")},
	}

	mainDecl := FuncDecl(
		nil,
		"main",
		nil,
		nil,
		[]ast.Stmt{regAssign, discardAssign},
	)

	packageDoc := singleComment(fmt.Sprintf(
		"// Command %s é o entrypoint %s (esqueleto gerado por scaffold entrypoint create).", name, name))

	packageDoc.List[0].Slash = padder.Take()
	packagePos := padder.Take()
	padder.Gap(1)
	// Imports já têm Pos atribuídos acima — o printer respeita Lparen/Rparen.
	// Forçar a Pos do GenDecl pra acompanhar.
	imports.TokPos = padder.Take()
	padder.Gap(1)
	mainDecl.Type.Func = padder.Take()
	mainDecl.Body.Lbrace = padder.Take()
	regLhs.NamePos = padder.Take()
	regAssign.TokPos = regLhs.NamePos
	discardAssign.TokPos = padder.Take()
	mainDecl.Body.Rbrace = padder.Take()

	file := &ast.File{
		Doc:     packageDoc,
		Package: packagePos,
		Name:    Ident("main"),
		Decls:   []ast.Decl{imports, mainDecl},
	}
	file.Comments = collectDocs(packageDoc, file.Decls)

	return finalizeASTSource(fset, file)
}

// finalizeASTSource formata o AST e roda gofmt em cima do resultado pra
// normalizar a saída — mesmo pipeline usado por repository_create/buildConcreteFile.
func finalizeASTSource(fset *token.FileSet, file *ast.File) ([]byte, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, fmt.Errorf("formatar AST: %w", err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w\n%s", err, buf.String())
	}
	if !bytes.HasSuffix(formatted, []byte("\n")) {
		formatted = append(formatted, '\n')
	}
	return formatted, nil
}

// stripUnderscores remove todos os underscores de s. Usado pra gerar aliases
// de import a partir de nomes em snake_case (ex.: "redis_queue" → "redisqueue"),
// mesma técnica do `repository register` que vira `<snake>repo` sem underscores.
func stripUnderscores(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '_' {
			out = append(out, s[i])
		}
	}
	return string(out)
}
