//go:build cgo

package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/scip/bindings/go/scip"
	sitter "github.com/tree-sitter/go-tree-sitter"
	tsjava "github.com/tree-sitter/tree-sitter-java/bindings/go"
	tsjavascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tspython "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tsrust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tstypescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

type TreeSitterIndexer struct{}

type treeSitterIndexCommand struct{}

func init() {
	RegisterCommand(treeSitterIndexCommand{})
}

// NewTreeSitterIndexer returns the cgo-backed tree-sitter fallback indexer.
func NewTreeSitterIndexer() *TreeSitterIndexer {
	return &TreeSitterIndexer{}
}

func (treeSitterIndexCommand) Name() string {
	return "tree-sitter"
}

func (treeSitterIndexCommand) Supports(language string, _ IndexRequest) bool {
	return language != "go"
}

func (treeSitterIndexCommand) Execute(ctx context.Context, req IndexRequest) (Result, error) {
	return NewTreeSitterIndexer().Index(ctx, req)
}

// Index parses supported non-Go files with tree-sitter and returns symbolic
// summaries plus an optional SCIP snapshot.
func (t *TreeSitterIndexer) Index(ctx context.Context, req IndexRequest) (Result, error) {
	req, err := NormalizeRequest(req)
	if err != nil {
		return Result{}, err
	}

	files, warnings, err := CollectFiles(req)
	if err != nil {
		return Result{}, err
	}

	sampled := false
	if len(files) > req.SampleLimit {
		files = files[:req.SampleLimit]
		sampled = true
	}

	summaries := make([]FileSummary, 0, len(files))
	documents := make([]*scip.Document, 0, len(files))
	skippedCount := 0

	for _, rel := range files {
		summary, doc, err := t.summarizeFile(ctx, req.Root, rel, req.SymbolicOnly)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipping %s: %v", rel, err))
			skippedCount++
			continue
		}
		if summary.SkipReason != "" {
			warnings = append(warnings, summary.SkipReason)
			skippedCount++
		}
		summaries = append(summaries, summary)
		if doc != nil {
			documents = append(documents, doc)
		}
	}

	outputPath := req.OutputPath
	if outputPath == "" && req.EmitSCIP {
		outputPath = filepath.Join(req.Root, "index.scip")
	}
	if req.EmitSCIP {
		if err := writeSCIPSnapshot(outputPath, req.Root, "scip-cli-tree-sitter", documents); err != nil {
			return Result{}, err
		}
	}

	language := req.Language
	if language == "" {
		language = "mixed"
	}

	return Result{
		Indexer:       "tree-sitter",
		Root:          req.Root,
		Language:      language,
		FilesScanned:  len(files),
		FilesIndexed:  len(summaries),
		FilesSkipped:  skippedCount,
		Sampled:       sampled,
		SCIPPath:      outputPath,
		FileSummaries: summaries,
		Warnings:      warnings,
	}, nil
}

func (t *TreeSitterIndexer) summarizeFile(ctx context.Context, root, rel string, symbolicOnly bool) (FileSummary, *scip.Document, error) {
	absolute := filepath.Join(root, rel)
	content, err := os.ReadFile(absolute)
	if err != nil {
		return FileSummary{}, nil, err
	}
	if warning := InvisibleUnicodeWarning(rel, content); warning != "" {
		return FileSummary{
			Path:       rel,
			Language:   DetectLanguage(rel),
			Bytes:      int64(len(content)),
			Skipped:    true,
			SkipReason: warning,
		}, nil, nil
	}

	language := DetectLanguage(rel)
	tsLang, err := grammarForFile(rel)
	if err != nil {
		symbols := fallbackSymbols(rel, content)
		return FileSummary{
			Path:         rel,
			Language:     language,
			Bytes:        int64(len(content)),
			Symbols:      symbols,
			SymbolicSExp: buildSExpression(rel, symbols),
		}, buildDocument(rel, language, content, symbols, symbolicOnly), nil
	}

	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(tsLang); err != nil {
		return FileSummary{}, nil, err
	}

	tree := parser.ParseWithOptions(func(offset int, _ sitter.Point) []byte {
		if offset < len(content) {
			return content[offset:]
		}
		return []byte{}
	}, nil, &sitter.ParseOptions{
		ProgressCallback: func(sitter.ParseState) bool {
			return ctx.Err() != nil
		},
	})
	if tree == nil {
		if err := ctx.Err(); err != nil {
			return FileSummary{}, nil, err
		}
		return FileSummary{}, nil, fmt.Errorf("tree-sitter returned no parse tree")
	}
	defer tree.Close()
	rootNode := tree.RootNode()
	if rootNode == nil {
		return FileSummary{}, nil, fmt.Errorf("tree-sitter returned no root node")
	}

	symbols := extractSymbols(rel, content, rootNode)
	doc := buildDocument(rel, language, content, symbols, symbolicOnly)

	return FileSummary{
		Path:         rel,
		Language:     language,
		Bytes:        int64(len(content)),
		Symbols:      symbols,
		SymbolicSExp: compactSExpression(rootNode),
	}, doc, nil
}

func fallbackSymbols(path string, content []byte) []Symbol {
	matches := simpleSymbolPattern.FindAllStringSubmatchIndex(string(content), -1)
	symbols := make([]Symbol, 0, len(matches))
	for _, match := range matches {
		name := string(content[match[2]:match[3]])
		line := 1 + strings.Count(string(content[:match[0]]), "\n")
		symbols = append(symbols, Symbol{
			Name:      name,
			Kind:      "symbol",
			Path:      path,
			StartLine: line,
			EndLine:   line,
		})
	}
	return symbols
}

func grammarForFile(path string) (*sitter.Language, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".jsx", ".cjs", ".mjs":
		return sitter.NewLanguage(tsjavascript.Language()), nil
	case ".ts", ".mts", ".cts":
		return sitter.NewLanguage(tstypescript.LanguageTypescript()), nil
	case ".tsx":
		return sitter.NewLanguage(tstypescript.LanguageTSX()), nil
	case ".py":
		return sitter.NewLanguage(tspython.Language()), nil
	case ".rs":
		return sitter.NewLanguage(tsrust.Language()), nil
	case ".java":
		return sitter.NewLanguage(tsjava.Language()), nil
	default:
		return nil, fmt.Errorf("no tree-sitter grammar configured for %s", path)
	}
}

func extractSymbols(path string, content []byte, root *sitter.Node) []Symbol {
	cursor := root.Walk()
	defer cursor.Close()

	queue := []*sitter.Node{root}
	symbols := make([]Symbol, 0, 16)

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if node == nil || !node.IsNamed() {
			continue
		}

		if symbol := nodeToSymbol(path, content, node); symbol.Name != "" {
			symbols = append(symbols, symbol)
		}

		children := node.NamedChildren(cursor)
		for i := range children {
			child := children[i]
			if child.StartPosition().Row > 4000 {
				continue
			}
			queue = append(queue, &child)
		}
	}

	return symbols
}

func nodeToSymbol(path string, content []byte, node *sitter.Node) Symbol {
	var (
		kind = node.Kind()
		name string
	)

	switch kind {
	case "function_declaration", "function_definition", "function_item", "method_definition", "method_declaration":
		name = childText(node, content, "name")
	case "class_declaration", "class_definition", "interface_declaration", "interface_definition", "enum_declaration", "type_alias_declaration", "type_definition", "struct_item", "impl_item":
		name = childText(node, content, "name")
	case "lexical_declaration", "variable_declaration", "variable_declarator", "assignment":
		name = childText(node, content, "name")
		if name == "" {
			name = childText(node, content, "left")
		}
	}

	if name == "" {
		return Symbol{}
	}

	start := node.StartPosition()
	end := node.EndPosition()
	return Symbol{
		Name:      name,
		Kind:      normalizeKind(kind),
		Path:      path,
		StartLine: int(start.Row) + 1,
		EndLine:   int(end.Row) + 1,
	}
}

func childText(node *sitter.Node, content []byte, field string) string {
	child := node.ChildByFieldName(field)
	if child == nil {
		return ""
	}
	return strings.TrimSpace(child.Utf8Text(content))
}

func normalizeKind(kind string) string {
	switch {
	case strings.Contains(kind, "function"), strings.Contains(kind, "method"):
		return "function"
	case strings.Contains(kind, "class"), strings.Contains(kind, "struct"), strings.Contains(kind, "interface"), strings.Contains(kind, "type"):
		return "type"
	case strings.Contains(kind, "enum"):
		return "enum"
	case strings.Contains(kind, "variable"), strings.Contains(kind, "assignment"):
		return "variable"
	default:
		return "symbol"
	}
}

func compactSExpression(root *sitter.Node) string {
	sexp := root.ToSexp()
	if len(sexp) > 480 {
		return sexp[:480] + "..."
	}
	return sexp
}

func buildDocument(path, language string, content []byte, symbols []Symbol, symbolicOnly bool) *scip.Document {
	doc := &scip.Document{
		RelativePath:     filepath.ToSlash(path),
		Language:         language,
		PositionEncoding: scip.PositionEncoding_UTF8CodeUnitOffsetFromLineStart,
	}
	if !symbolicOnly {
		doc.Text = string(content)
	}

	for index, symbol := range symbols {
		symbolID := formatSymbol(language, symbol, index)
		doc.Symbols = append(doc.Symbols, &scip.SymbolInformation{
			Symbol:      symbolID,
			DisplayName: symbol.Name,
			Kind:        symbolKind(symbol.Kind),
		})
		doc.Occurrences = append(doc.Occurrences, &scip.Occurrence{
			Range:       []int32{int32(symbol.StartLine - 1), 0, int32(symbol.EndLine), 0},
			Symbol:      symbolID,
			SymbolRoles: int32(scip.SymbolRole_Definition),
		})
	}

	return doc
}
