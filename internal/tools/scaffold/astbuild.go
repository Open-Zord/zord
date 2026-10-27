// Package scaffold (área astbuild) oferece builders genéricos pra construir nós go/ast e
// controlar posições (token.Pos) sem precisar montar literais verbosos em
// cada gerador. Usado pelos subpacotes do scaffold (domain, repository, ...)
// para manter a regra do projeto: toda geração de código Go é via AST puro.
package scaffold

import (
	"go/ast"
	"go/token"
	"strconv"
)

// LinePadder gera token.Pos em linhas distintas dentro de um token.File
// sintético no FileSet alvo. Necessário porque o go/printer só insere linha
// em branco entre nós (Decls top-level, ImportSpecs num bloco, Elts de um
// CompositeLit) cujas Pos estejam em linhas separadas no FileSet; nós
// construídos manualmente têm Pos zero e saem colados.
type LinePadder struct {
	file *token.File
	line int // próxima linha disponível (1-based)
}

// NewLinePadder cria um token.File sintético no fset com várias linhas
// pré-declaradas (offsets espaçados) para servir de "pool" de posições.
func NewLinePadder(fset *token.FileSet, name string) *LinePadder {
	const (
		stride  = 16
		nLines  = 1 << 14
		fileLen = stride * nLines
	)
	f := fset.AddFile(name, -1, fileLen)
	starts := make([]int, nLines)
	for i := range starts {
		starts[i] = i * stride
	}
	f.SetLines(starts)
	return &LinePadder{file: f, line: 1}
}

// Take reserva a próxima linha e retorna sua Pos.
func (p *LinePadder) Take() token.Pos {
	pos := p.file.LineStart(p.line)
	p.line++
	return pos
}

// Gap pula n linhas sem retornar Pos. Cada linha pulada vira uma linha em
// branco no output do printer entre os nós que delimitam o gap.
func (p *LinePadder) Gap(n int) {
	p.line += n
}

// StampDecls atribui Pos top-level a uma sequência de Decls, com gap=1 entre
// cada par (uma linha em branco entre Decls no output). Para FuncDecls
// também atribui Pos distintas a Body.Lbrace/Rbrace pra forçar multi-line
// mesmo em corpos curtos.
func (p *LinePadder) StampDecls(decls ...ast.Decl) {
	for i, d := range decls {
		if i > 0 {
			p.Gap(1)
		}
		switch x := d.(type) {
		case *ast.FuncDecl:
			x.Type.Func = p.Take()
			if x.Body != nil {
				x.Body.Lbrace = p.Take()
				x.Body.Rbrace = p.Take()
			}
		case *ast.GenDecl:
			x.TokPos = p.Take()
		}
	}
}

// --- AST builders genéricos ---

// Ident retorna *ast.Ident para o nome dado.
func Ident(s string) *ast.Ident { return ast.NewIdent(s) }

// Sel monta uma SelectorExpr "x.s".
func Sel(x, s string) *ast.SelectorExpr {
	return &ast.SelectorExpr{X: Ident(x), Sel: Ident(s)}
}

// StarOf monta "*T".
func StarOf(t ast.Expr) *ast.StarExpr { return &ast.StarExpr{X: t} }

// StrLit monta uma BasicLit string já quotada via strconv.Quote.
func StrLit(s string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(s)}
}

// Field monta um *ast.Field nomeado: `name typ`.
func Field(name string, typ ast.Expr) *ast.Field {
	return &ast.Field{Names: []*ast.Ident{Ident(name)}, Type: typ}
}

// AnonField monta um *ast.Field sem nome (usado em result lists, embeds, e
// methods de interface).
func AnonField(typ ast.Expr) *ast.Field { return &ast.Field{Type: typ} }

// FieldList monta uma *ast.FieldList a partir de zero ou mais fields.
func FieldList(fs ...*ast.Field) *ast.FieldList {
	if len(fs) == 0 {
		return &ast.FieldList{}
	}
	return &ast.FieldList{List: fs}
}

// ValueReceiver monta `(recv TypeName)`.
func ValueReceiver(recv, typeName string) *ast.FieldList {
	return FieldList(Field(recv, Ident(typeName)))
}

// PointerReceiver monta `(recv *TypeName)`.
func PointerReceiver(recv, typeName string) *ast.FieldList {
	return FieldList(Field(recv, StarOf(Ident(typeName))))
}

// FuncDecl monta um *ast.FuncDecl completo. Se params for nil, usa
// FieldList vazia. Body sempre não-nil.
func FuncDecl(recv *ast.FieldList, name string, params, results *ast.FieldList, body []ast.Stmt) *ast.FuncDecl {
	if params == nil {
		params = &ast.FieldList{}
	}
	return &ast.FuncDecl{
		Recv: recv,
		Name: Ident(name),
		Type: &ast.FuncType{Params: params, Results: results},
		Body: &ast.BlockStmt{List: body},
	}
}

// ReturnStmt monta `return <exprs...>`.
func ReturnStmt(exprs ...ast.Expr) *ast.ReturnStmt {
	return &ast.ReturnStmt{Results: exprs}
}

// Assign monta `lhs = rhs` (assignment, não declaração).
func Assign(lhs, rhs ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: []ast.Expr{lhs}, Tok: token.ASSIGN, Rhs: []ast.Expr{rhs}}
}

// IfStmt monta `if cond { body }` sem else.
func IfStmt(cond ast.Expr, body []ast.Stmt) *ast.IfStmt {
	return &ast.IfStmt{Cond: cond, Body: &ast.BlockStmt{List: body}}
}

// Binary monta `x op y`.
func Binary(op token.Token, x, y ast.Expr) *ast.BinaryExpr {
	return &ast.BinaryExpr{X: x, Op: op, Y: y}
}

// IndexExpr monta `x[index]` (também serve para instanciação de genéricos
// com um único type argument, ex.: BaseRepo[Foo]).
func IndexExpr(x, index ast.Expr) *ast.IndexExpr {
	return &ast.IndexExpr{X: x, Index: index}
}

// CompositeLit monta `T{}` sem elements. Para elements, atribuir após:
//
//	cl := CompositeLit(typ)
//	cl.Elts = []ast.Expr{...}
func CompositeLit(typ ast.Expr) *ast.CompositeLit {
	return &ast.CompositeLit{Type: typ}
}

// ImportSpec monta um *ast.ImportSpec com o path quotado.
func ImportSpec(path string) *ast.ImportSpec {
	return &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)}}
}

// ImportGroups monta um único GenDecl(IMPORT) paren-block contendo vários
// grupos de paths separados por linha em branco no output. Atribui Pos a
// cada ImportSpec via padder pra controlar o agrupamento que o printer
// reconhece (gap entre Pos.Line → blank line).
func ImportGroups(padder *LinePadder, groups ...[]string) *ast.GenDecl {
	lparen := padder.Take()
	var specs []ast.Spec
	for gi, paths := range groups {
		if gi > 0 {
			padder.Gap(1)
		}
		for _, p := range paths {
			spec := ImportSpec(p)
			spec.Path.ValuePos = padder.Take()
			specs = append(specs, spec)
		}
	}
	rparen := padder.Take()
	return &ast.GenDecl{
		Tok:    token.IMPORT,
		Lparen: lparen,
		Rparen: rparen,
		Specs:  specs,
	}
}

// TypeDecl monta `type Name <typ>` como GenDecl(TYPE).
func TypeDecl(name string, typ ast.Expr) *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{Name: Ident(name), Type: typ},
		},
	}
}

// StampCompositeLit força o CompositeLit a sair multi-line, atribuindo
// Pos distintas a Lbrace, ao primeiro nó posicionável de cada Elt e ao
// Rbrace. Sem isso, o printer cola tudo numa única linha quando o conteúdo
// cabe.
func StampCompositeLit(padder *LinePadder, cl *ast.CompositeLit) {
	cl.Lbrace = padder.Take()
	for _, e := range cl.Elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok {
				id.NamePos = padder.Take()
				continue
			}
		}
		// fallback: tenta atribuir Pos genérica em Elts não-KV (raro).
		padder.Take()
	}
	cl.Rbrace = padder.Take()
}
