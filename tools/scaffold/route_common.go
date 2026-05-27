package scaffold

import (
	"go/ast"
	"go/token"
)

// stampCompositeLitDeep força layout multi-line num CompositeLit com KVs
// cujos Values são *ast.CallExpr. Diferente do StampCompositeLit
// (que só seta Key.NamePos), também atribui Colon, Lparen, Rparen e a Pos
// do primeiro arg da call — sem isso o go/printer cola múltiplos KVs
// numa única linha (ou pior, quebra a chamada com newlines internas)
// porque os args sem Pos resolvem para linha 0, e a heurística do
// printer decide separar Lparen do conteúdo.
//
// Cada KV recebe uma linha distinta; Lbrace fica antes e Rbrace depois.
// Layout final: cada `<key>: <call>(<args>)` numa linha própria, separados
// por vírgula + newline.
func stampCompositeLitDeep(padder *LinePadder, cl *ast.CompositeLit) {
	cl.Lbrace = padder.Take()
	for _, e := range cl.Elts {
		kvLine := padder.Take()
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if id, ok := kv.Key.(*ast.Ident); ok {
			id.NamePos = kvLine
		}
		kv.Colon = kvLine
		ce, ok := kv.Value.(*ast.CallExpr)
		if !ok {
			continue
		}
		ce.Lparen = kvLine
		ce.Rparen = kvLine
		// Posicionar todos os args na mesma linha do KV — sem isso,
		// args sem Pos (SelectorExprs montados manualmente) resolvem
		// para linha 0 e o printer quebra a chamada em múltiplas linhas
		// internas.
		for _, arg := range ce.Args {
			stampExprLine(arg, kvLine)
		}
	}
	cl.Rbrace = padder.Take()
}

// stampExprLine atribui Pos = `line` a todos os nós posicionáveis dentro
// de `expr`. Cobre os args da chamada
// `registry.Resolve[*<svc>.<Pascal>Handler](reg, <svc>.RegistryKey)`:
// *ast.Ident e *ast.SelectorExpr (precisa setar tanto X quanto Sel — sem
// o Sel, o printer trata `Sel.Pos == 0` como "linha anterior" e quebra a
// expressão).
func stampExprLine(expr ast.Expr, line token.Pos) {
	switch e := expr.(type) {
	case *ast.Ident:
		e.NamePos = line
	case *ast.SelectorExpr:
		stampExprLine(e.X, line)
		if e.Sel != nil {
			e.Sel.NamePos = line
		}
	}
}
