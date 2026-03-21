package indexer

import (
	"bufio"
	"encoding/json"
	"strings"
)

type rawDocumentSymbol struct {
	Name           string              `json:"name"`
	Detail         string              `json:"detail,omitempty"`
	Kind           int                 `json:"kind"`
	Range          lspRange            `json:"range"`
	SelectionRange lspRange            `json:"selectionRange"`
	Children       []rawDocumentSymbol `json:"children"`
}

type rawSymbolInformation struct {
	Name     string      `json:"name"`
	Detail   string      `json:"detail,omitempty"`
	Kind     int         `json:"kind"`
	Location lspLocation `json:"location"`
}

func decodeDocumentSymbols(path, source string, payload json.RawMessage) ([]Symbol, []lspSymbolRef, error) {
	var rawItems []json.RawMessage
	if err := json.Unmarshal(payload, &rawItems); err != nil {
		return nil, nil, err
	}

	lines := splitSourceLines(source)
	symbols := make([]Symbol, 0, len(rawItems))
	refs := make([]lspSymbolRef, 0, len(rawItems))
	for _, rawItem := range rawItems {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(rawItem, &probe); err != nil {
			return nil, nil, err
		}
		// LSP servers may return either hierarchical DocumentSymbol values or the
		// older flat SymbolInformation variant for this request.
		if _, ok := probe["selectionRange"]; ok {
			var item rawDocumentSymbol
			if err := json.Unmarshal(rawItem, &item); err != nil {
				return nil, nil, err
			}
			flattenDocumentSymbols(path, lines, item, &symbols, &refs)
			continue
		}

		var item rawSymbolInformation
		if err := json.Unmarshal(rawItem, &item); err != nil {
			return nil, nil, err
		}
		symbols = append(symbols, Symbol{
			Name:      item.Name,
			Kind:      normalizeLSPKind(item.Kind),
			Path:      path,
			StartLine: item.Location.Range.Start.Line + 1,
			EndLine:   max(item.Location.Range.End.Line+1, item.Location.Range.Start.Line+1),
			Signature: symbolSignature(item.Detail, lines, item.Location.Range.Start.Line),
			BodyText:  symbolBody(lines, item.Location.Range),
		})
		refs = append(refs, lspSymbolRef{
			Name:     item.Name,
			Kind:     normalizeLSPKind(item.Kind),
			Range:    item.Location.Range,
			Position: item.Location.Range.Start,
		})
	}
	return symbols, refs, nil
}

func flattenDocumentSymbols(path string, lines []string, item rawDocumentSymbol, symbols *[]Symbol, refs *[]lspSymbolRef) {
	position := item.SelectionRange.Start
	if item.SelectionRange == (lspRange{}) {
		position = item.Range.Start
	}
	*symbols = append(*symbols, Symbol{
		Name:      item.Name,
		Kind:      normalizeLSPKind(item.Kind),
		Path:      path,
		StartLine: position.Line + 1,
		EndLine:   max(item.Range.End.Line+1, position.Line+1),
		Signature: symbolSignature(item.Detail, lines, position.Line),
		BodyText:  symbolBody(lines, item.Range),
	})
	*refs = append(*refs, lspSymbolRef{
		Name:     item.Name,
		Kind:     normalizeLSPKind(item.Kind),
		Range:    item.Range,
		Position: position,
	})
	for _, child := range item.Children {
		flattenDocumentSymbols(path, lines, child, symbols, refs)
	}
}

func symbolSignature(detail string, lines []string, line int) string {
	if strings.TrimSpace(detail) != "" {
		return strings.TrimSpace(detail)
	}
	if line < 0 || line >= len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[line])
}

func symbolBody(lines []string, symbolRange lspRange) string {
	if len(lines) == 0 {
		return ""
	}
	start := max(symbolRange.Start.Line, 0)
	if start >= len(lines) {
		return ""
	}
	end := max(symbolRange.End.Line, start)
	if end >= len(lines) {
		end = len(lines) - 1
	}
	if end-start > 39 {
		end = start + 39
	}
	block := make([]string, 0, end-start+1)
	for line := start; line <= end; line++ {
		block = append(block, lines[line])
	}
	return strings.TrimSpace(strings.Join(block, "\n"))
}

func splitSourceLines(source string) []string {
	scanner := bufio.NewScanner(strings.NewReader(source))
	// LSP symbol extraction already has the full document text in memory, so we
	// can raise the scanner limit to the document size and let ScanLines
	// normalize LF and CRLF terminators consistently.
	scanner.Buffer(make([]byte, 0, min(len(source), 64*1024)), max(len(source), 64*1024))

	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		// Scanning an in-memory string should not fail in practice, so keep the
		// original payload as a single line rather than returning partial text.
		return []string{source}
	}

	return lines
}

func normalizeLSPKind(kind int) string {
	switch kind {
	case 5, 6, 12, 22:
		return "type"
	case 10, 11, 23:
		return "enum"
	case 13, 14:
		return "variable"
	default:
		return "function"
	}
}
