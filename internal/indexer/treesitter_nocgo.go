//go:build !cgo

package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type TreeSitterIndexer struct{}

type treeSitterIndexCommand struct{}

func init() {
	RegisterCommand(treeSitterIndexCommand{})
}

// NewTreeSitterIndexer returns the non-Go indexer implementation for the
// current build configuration.
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

// Index performs compact symbolic indexing without requiring cgo-backed
// tree-sitter grammars.
func (t *TreeSitterIndexer) Index(_ context.Context, req IndexRequest) (Result, error) {
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
	skippedCount := 0
	for _, rel := range files {
		summary, err := t.summarizeFile(req.Root, rel)
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
	}

	warnings = append(warnings, "tree-sitter cgo grammars are unavailable in this environment; using a lightweight symbolic fallback")

	return Result{
		Indexer:       "symbolic-fallback",
		Root:          req.Root,
		Language:      firstNonEmpty(req.Language, "mixed"),
		FilesScanned:  len(files),
		FilesIndexed:  len(summaries),
		FilesSkipped:  skippedCount,
		Sampled:       sampled,
		FileSummaries: summaries,
		Warnings:      warnings,
	}, nil
}

func (t *TreeSitterIndexer) summarizeFile(root, rel string) (FileSummary, error) {
	absolute := filepath.Join(root, rel)
	content, err := os.ReadFile(absolute)
	if err != nil {
		return FileSummary{}, err
	}
	if warning := InvisibleUnicodeWarning(rel, content); warning != "" {
		return FileSummary{
			Path:       rel,
			Language:   DetectLanguage(rel),
			Bytes:      int64(len(content)),
			Skipped:    true,
			SkipReason: warning,
		}, nil
	}

	matches := simpleSymbolPattern.FindAllStringSubmatchIndex(string(content), -1)
	symbols := make([]Symbol, 0, len(matches))
	for _, match := range matches {
		name := string(content[match[2]:match[3]])
		line := 1 + strings.Count(string(content[:match[0]]), "\n")
		symbols = append(symbols, Symbol{
			Name:      name,
			Kind:      "symbol",
			Path:      rel,
			StartLine: line,
			EndLine:   line,
		})
	}

	return FileSummary{
		Path:         rel,
		Language:     DetectLanguage(rel),
		Bytes:        int64(len(content)),
		Symbols:      symbols,
		SymbolicSExp: buildSExpression(rel, symbols),
	}, nil
}
