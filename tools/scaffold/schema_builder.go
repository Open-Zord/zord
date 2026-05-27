package scaffold

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// buildTable constrói o bloco `table "<table>" { ... }` em HCL via hclwrite
// a partir das FieldSpecs. Retorna os bytes formatados do bloco (sem sentinela).
func buildTable(table, schemaName string, fields []SchemaFieldSpec) ([]byte, error) {
	f := hclwrite.NewEmptyFile()
	tb := f.Body().AppendNewBlock("table", []string{table})
	body := tb.Body()

	body.SetAttributeTraversal("schema", hcl.Traversal{
		hcl.TraverseRoot{Name: "schema"},
		hcl.TraverseAttr{Name: schemaName},
	})

	var (
		pkCols  []SchemaFieldSpec
		idxCols []SchemaFieldSpec
		fkCols  []SchemaFieldSpec
	)
	for _, fs := range fields {
		if err := writeColumn(body, fs); err != nil {
			return nil, fmt.Errorf("coluna %s: %w", fs.DBName, err)
		}
		if fs.DBPK {
			pkCols = append(pkCols, fs)
		}
		if fs.DBIndex == "true" || fs.DBIndex == "unique" {
			idxCols = append(idxCols, fs)
		}
		if fs.DBFK != "" {
			fkCols = append(fkCols, fs)
		}
	}

	if len(pkCols) > 0 {
		pk := body.AppendNewBlock("primary_key", nil)
		cols := make([]hclwrite.Tokens, 0, len(pkCols))
		for _, c := range pkCols {
			cols = append(cols, columnRefTokens(c.DBName))
		}
		pk.Body().SetAttributeRaw("columns", hclwrite.TokensForTuple(cols))
	}

	for _, fs := range idxCols {
		unique := fs.DBIndex == "unique"
		ib := body.AppendNewBlock("index", []string{indexBlockName(table, fs.DBName, unique)})
		ib.Body().SetAttributeRaw("columns", hclwrite.TokensForTuple([]hclwrite.Tokens{columnRefTokens(fs.DBName)}))
		if unique {
			ib.Body().SetAttributeValue("unique", cty.True)
		}
	}

	for _, fs := range fkCols {
		refTable, refCol, err := parseFK(fs.DBFK)
		if err != nil {
			return nil, fmt.Errorf("campo %s: %w", fs.Name, err)
		}
		fkb := body.AppendNewBlock("foreign_key", []string{fkBlockName(table, fs.DBName)})
		fkb.Body().SetAttributeRaw("columns", hclwrite.TokensForTuple([]hclwrite.Tokens{columnRefTokens(fs.DBName)}))
		fkb.Body().SetAttributeRaw("ref_columns", hclwrite.TokensForTuple([]hclwrite.Tokens{
			tableColumnRefTokens(refTable, refCol),
		}))
		fkb.Body().SetAttributeRaw("on_delete", identTokens("CASCADE"))
	}

	return hclwrite.Format(f.Bytes()), nil
}

func writeColumn(parent *hclwrite.Body, fs SchemaFieldSpec) error {
	c := parent.AppendNewBlock("column", []string{fs.DBName})
	cb := c.Body()
	typeTokens, err := typeTokens(fs)
	if err != nil {
		return err
	}
	cb.SetAttributeRaw("type", typeTokens)
	cb.SetAttributeValue("null", cty.BoolVal(fs.Pointer))
	return nil
}

func typeTokens(fs SchemaFieldSpec) (hclwrite.Tokens, error) {
	if fs.DBType != "" {
		if fs.DBSize == "" {
			return identTokens(fs.DBType), nil
		}
		return funcCallTokens(fs.DBType, fs.DBSize), nil
	}
	switch fs.GoType {
	case "string":
		size := fs.DBSize
		if size == "" {
			size = "255"
		}
		return funcCallTokens("varchar", size), nil
	case "int64":
		return identTokens("bigint"), nil
	case "int32", "int":
		return identTokens("int"), nil
	case "bool":
		return funcCallTokens("tinyint", "1"), nil
	case "time.Time":
		return identTokens("datetime"), nil
	}
	return nil, fmt.Errorf("tipo Go %q sem mapeamento default — use db_type", fs.GoType)
}

func columnRefTokens(col string) hclwrite.Tokens {
	return hclwrite.TokensForTraversal(hcl.Traversal{
		hcl.TraverseRoot{Name: "column"},
		hcl.TraverseAttr{Name: col},
	})
}

func tableColumnRefTokens(table, col string) hclwrite.Tokens {
	return hclwrite.TokensForTraversal(hcl.Traversal{
		hcl.TraverseRoot{Name: "table"},
		hcl.TraverseAttr{Name: table},
		hcl.TraverseAttr{Name: "column"},
		hcl.TraverseAttr{Name: col},
	})
}

func identTokens(s string) hclwrite.Tokens {
	return hclwrite.Tokens{{Type: hclsyntax.TokenIdent, Bytes: []byte(s)}}
}

// funcCallTokens monta `name(arg1, arg2, ...)` aceitando args separados por vírgula
// num único string (ex.: "16,6" → decimal(16,6)). Espaços em volta dos args são aparados.
func funcCallTokens(name, argsCSV string) hclwrite.Tokens {
	parts := strings.Split(argsCSV, ",")
	args := make([]hclwrite.Tokens, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		args = append(args, hclwrite.Tokens{{Type: hclsyntax.TokenNumberLit, Bytes: []byte(p)}})
	}
	return hclwrite.TokensForFunctionCall(name, args...)
}

func parseFK(ref string) (table, col string, err error) {
	parts := strings.SplitN(ref, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("db_fk inválido %q: esperado <tabela>.<coluna>", ref)
	}
	return parts[0], parts[1], nil
}

func indexBlockName(table, column string, unique bool) string {
	if unique {
		return "idx_" + table + "_" + column + "_uq"
	}
	return "idx_" + table + "_" + column
}

func fkBlockName(table, column string) string {
	return "fk_" + table + "_" + column
}
