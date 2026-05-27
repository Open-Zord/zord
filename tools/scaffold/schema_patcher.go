package scaffold

import (
	"bytes"
	"fmt"
	"regexp"
)

var (
	rxSentinelGen = regexp.MustCompile(`^[ \t]*#[ \t]*scaffold:generated[ \t]+(\S+)[ \t]*$`)
	rxSentinelEnd = regexp.MustCompile(`^[ \t]*#[ \t]*scaffold:end[ \t]+(\S+)[ \t]*$`)
	rxTableDecl   = regexp.MustCompile(`^[ \t]*table[ \t]+"([^"]+)"[ \t]*\{`)
)

// patch substitui o bloco do `table` envolvido pelas sentinelas
// `# scaffold:generated <table>` / `# scaffold:end <table>` em raw, ou anexa
// um bloco novo se não houver sentinela. Conteúdo fora das sentinelas é
// preservado byte-a-byte. Falha em sentinela parcial ou em `table "<table>"`
// pré-existente fora de sentinela.
func patch(raw []byte, table string, block []byte) ([]byte, error) {
	pairs, err := scanSentinels(raw)
	if err != nil {
		return nil, err
	}

	if line := findUnwrappedTable(raw, pairs, table); line >= 0 {
		return nil, fmt.Errorf("table %q já existe (linha %d) sem sentinela — criada à mão, recuso sobrescrever", table, line+1)
	}

	sentinelStart := "# scaffold:generated " + table
	sentinelEnd := "# scaffold:end " + table
	wrapped := buildWrapped(sentinelStart, sentinelEnd, block)

	if existing, ok := pairs[table]; ok {
		return replaceRange(raw, existing.startByte, existing.endByte, wrapped), nil
	}
	return appendBlock(raw, wrapped), nil
}

type sentinelSpan struct {
	startByte int // posição do '#' da linha :generated
	endByte   int // posição logo após o '\n' que termina a linha :end (ou len(raw) se não há)
	name      string
}

// scanSentinels identifica todos os pares :generated/:end no arquivo,
// detecta inconsistências (sentinela órfã, duplicada, ou :end antes de :generated)
// e devolve um mapa name → span. Garante: para cada name, exatamente um par íntegro.
func scanSentinels(raw []byte) (map[string]sentinelSpan, error) {
	out := map[string]sentinelSpan{}
	type open struct {
		name       string
		startByte  int
		lineNumber int
	}
	var pending *open

	lineNo := 0
	pos := 0
	for pos <= len(raw) {
		lineNo++
		nl := bytes.IndexByte(raw[pos:], '\n')
		var lineEnd int
		var nextPos int
		if nl < 0 {
			lineEnd = len(raw)
			nextPos = len(raw) + 1 // breaks loop
		} else {
			lineEnd = pos + nl
			nextPos = pos + nl + 1
		}
		line := raw[pos:lineEnd]

		if m := rxSentinelGen.FindSubmatch(line); m != nil {
			name := string(m[1])
			if _, dup := out[name]; dup {
				return nil, fmt.Errorf("linha %d: sentinela :generated %q duplicada", lineNo, name)
			}
			if pending != nil {
				return nil, fmt.Errorf("linha %d: sentinela :generated %q aninhada — %q ainda não foi fechada (linha %d)",
					lineNo, name, pending.name, pending.lineNumber)
			}
			pending = &open{name: name, startByte: pos, lineNumber: lineNo}
		} else if m := rxSentinelEnd.FindSubmatch(line); m != nil {
			name := string(m[1])
			if pending == nil || pending.name != name {
				return nil, fmt.Errorf("linha %d: sentinela :end %q sem :generated pareada", lineNo, name)
			}
			out[name] = sentinelSpan{
				startByte: pending.startByte,
				endByte:   nextPos,
				name:      name,
			}
			pending = nil
		}

		pos = nextPos
	}

	if pending != nil {
		return nil, fmt.Errorf("linha %d: sentinela :generated %q sem :end pareada", pending.lineNumber, pending.name)
	}
	return out, nil
}

// findUnwrappedTable retorna o número da linha (0-based) de uma declaração
// `table "<name>" {` que NÃO esteja entre sentinelas, ou -1 se não houver.
func findUnwrappedTable(raw []byte, pairs map[string]sentinelSpan, name string) int {
	pos := 0
	lineIdx := 0
	for pos <= len(raw) {
		nl := bytes.IndexByte(raw[pos:], '\n')
		var lineEnd, nextPos int
		if nl < 0 {
			lineEnd = len(raw)
			nextPos = len(raw) + 1
		} else {
			lineEnd = pos + nl
			nextPos = pos + nl + 1
		}
		line := raw[pos:lineEnd]
		if m := rxTableDecl.FindSubmatch(line); len(m) > 1 && string(m[1]) == name {
			if !insideAnySentinel(pos, pairs) {
				return lineIdx
			}
		}
		pos = nextPos
		lineIdx++
	}
	return -1
}

func insideAnySentinel(bytePos int, pairs map[string]sentinelSpan) bool {
	for _, p := range pairs {
		if bytePos >= p.startByte && bytePos < p.endByte {
			return true
		}
	}
	return false
}

func buildWrapped(startMarker, endMarker string, block []byte) []byte {
	var out bytes.Buffer
	out.WriteString(startMarker)
	out.WriteByte('\n')
	out.Write(bytes.TrimRight(block, "\n"))
	out.WriteByte('\n')
	out.WriteString(endMarker)
	out.WriteByte('\n')
	return out.Bytes()
}

// unpatch remove o bloco do `table` envolvido pelas sentinelas
// `# scaffold:generated <table>` / `# scaffold:end <table>` em raw,
// preservando o restante do arquivo byte-a-byte. Falha se a sentinela
// estiver ausente ou parcial.
func unpatch(raw []byte, table string) ([]byte, error) {
	pairs, err := scanSentinels(raw)
	if err != nil {
		return nil, err
	}
	span, ok := pairs[table]
	if !ok {
		return nil, fmt.Errorf("sentinela ausente para table %q", table)
	}
	return replaceRange(raw, span.startByte, span.endByte, nil), nil
}

// replaceRange substitui raw[start:end] por replacement preservando o restante byte-a-byte.
func replaceRange(raw []byte, start, end int, replacement []byte) []byte {
	out := make([]byte, 0, len(raw)-(end-start)+len(replacement))
	out = append(out, raw[:start]...)
	out = append(out, replacement...)
	out = append(out, raw[end:]...)
	return out
}

// appendBlock anexa wrapped ao final de raw garantindo separação por linha em branco.
func appendBlock(raw, wrapped []byte) []byte {
	var out bytes.Buffer
	out.Write(raw)
	if !bytes.HasSuffix(raw, []byte{'\n'}) {
		out.WriteByte('\n')
	}
	if len(raw) > 0 {
		out.WriteByte('\n')
	}
	out.Write(wrapped)
	return out.Bytes()
}
