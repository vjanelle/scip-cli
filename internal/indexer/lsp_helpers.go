package indexer

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/scip/bindings/go/scip"
)

func closeLSPClient(client lspClient) {
	closeCtx, cancel := context.WithTimeout(context.Background(), defaultCommandTimeout)
	defer cancel()
	_ = client.Close(closeCtx)
}

func addOccurrence(seen map[string]struct{}, doc *scip.Document, occurrence *scip.Occurrence) {
	// SCIP protobuf messages embed state with a mutex, so keep occurrences on
	// the heap and dedupe by their fields instead of copying the struct by value.
	key := fmt.Sprintf("%s:%d:%v", occurrence.Symbol, occurrence.SymbolRoles, occurrence.Range)
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	doc.Occurrences = append(doc.Occurrences, occurrence)
}

func symbolRange(symbol Symbol) []int32 {
	startLine := max(symbol.StartLine-1, 0)
	endLine := max(symbol.EndLine-1, startLine)
	return []int32{int32(startLine), 0, int32(endLine), 0}
}

func rangeWithinSymbol(symbol Symbol, candidate lspRange) bool {
	startLine := max(symbol.StartLine-1, 0)
	endLine := max(symbol.EndLine-1, startLine)
	if candidate.Start.Line < startLine || candidate.End.Line > endLine {
		return false
	}
	return candidate.Start.Line > startLine || candidate.End.Line < endLine || candidate.Start.Character > 0 || candidate.End.Character > 0
}

func detectedLanguage(req IndexRequest, rel string) string {
	if req.Language != "" {
		return req.Language
	}
	if detected := DetectLanguage(rel); detected != "" {
		return detected
	}
	return "plaintext"
}

func uriForPath(root, rel string) string {
	return rootURI(filepath.Join(root, rel))
}

func rootURI(path string) string {
	slashed := filepath.ToSlash(path)
	if len(slashed) >= 2 && slashed[1] == ':' {
		slashed = "/" + slashed
	}
	return (&url.URL{
		Scheme: "file",
		Path:   slashed,
	}).String()
}

func relativePathFromURI(root, rawURI string) (string, bool) {
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return "", false
	}
	path := parsed.Path
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	rel, err := filepath.Rel(root, filepath.Clean(path))
	if err != nil {
		return "", false
	}
	if strings.HasPrefix(rel, "..") {
		return "", false
	}
	return rel, true
}

func sameRange(left, right lspRange) bool {
	return left.Start == right.Start && left.End == right.End
}

func (r lspRange) toSCIP() []int32 {
	return []int32{
		int32(r.Start.Line),
		int32(r.Start.Character),
		int32(r.End.Line),
		int32(r.End.Character),
	}
}
