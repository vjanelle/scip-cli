package indexer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sourcegraph/scip/bindings/go/scip"
)

type lspIndexCommand struct{}

// LSPIndexer builds SCIP output by driving a language server over stdio.
type LSPIndexer struct {
	newClient func(context.Context, IndexRequest) (lspClient, []string, error)
}

type lspClient interface {
	Initialize(context.Context, string, map[string]any) error
	OpenDocument(context.Context, string, string, string) error
	DocumentSymbols(context.Context, string, string, string) ([]Symbol, []lspSymbolRef, error)
	References(context.Context, string, lspPosition) ([]lspLocation, error)
	Close(context.Context) error
}

type lspSymbolRef struct {
	Name     string
	Kind     string
	Range    lspRange
	Position lspPosition
}

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type lspLocation struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

func init() {
	RegisterCommand(lspIndexCommand{})
}

// NewLSPIndexer constructs the opt-in generic LSP backend.
func NewLSPIndexer() *LSPIndexer {
	return &LSPIndexer{newClient: startLSPClient}
}

func (lspIndexCommand) Name() string {
	return "scip-lsp"
}

func (lspIndexCommand) Supports(_ string, req IndexRequest) bool {
	return req.LSPCommand != ""
}

func (lspIndexCommand) Execute(ctx context.Context, req IndexRequest) (Result, error) {
	return NewLSPIndexer().Index(ctx, req)
}

// Index opens files in the configured language server, queries document
// symbols and references, and emits a workspace-local SCIP snapshot.
func (l *LSPIndexer) Index(ctx context.Context, req IndexRequest) (Result, error) {
	req, err := NormalizeRequest(req)
	if err != nil {
		return Result{}, err
	}
	if req.LSPCommand == "" {
		return Result{}, errors.New("lspCommand is required when using the scip-lsp indexer")
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

	client, commandLine, err := l.newClient(ctx, req)
	if err != nil {
		return Result{}, err
	}
	defer closeLSPClient(client)

	if err := client.Initialize(ctx, rootURI(req.Root), req.LSPInitOptions); err != nil {
		return Result{}, fmt.Errorf("initialize lsp client: %w", err)
	}

	outputPath := req.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join(req.Root, "index.scip")
	}

	type openedFile struct {
		path     string
		language string
		content  string
	}
	opened := make([]openedFile, 0, len(files))
	summaries := make([]FileSummary, 0, len(files))
	symbolRefsByFile := map[string][]lspSymbolRef{}
	documents := map[string]*scip.Document{}
	occurrences := map[string]map[string]struct{}{}
	filesSkipped := 0
	referenceWarnings := map[string]struct{}{}

	for _, rel := range files {
		absolute := filepath.Join(req.Root, rel)
		content, readErr := os.ReadFile(absolute)
		if readErr != nil {
			warnings = append(warnings, "failed to inspect "+filepath.ToSlash(rel)+": "+readErr.Error())
			filesSkipped++
			continue
		}
		if warning := InvisibleUnicodeWarning(rel, content); warning != "" {
			warnings = append(warnings, warning)
			filesSkipped++
			summaries = append(summaries, FileSummary{
				Path:       rel,
				Language:   detectedLanguage(req, rel),
				Bytes:      int64(len(content)),
				Skipped:    true,
				SkipReason: warning,
			})
			continue
		}

		language := detectedLanguage(req, rel)
		text := string(content)
		if err := client.OpenDocument(ctx, uriForPath(req.Root, rel), language, text); err != nil {
			return Result{}, fmt.Errorf("open %s in lsp: %w", filepath.ToSlash(rel), err)
		}
		opened = append(opened, openedFile{path: rel, language: language, content: text})
	}

	for _, file := range opened {
		symbols, refs, err := client.DocumentSymbols(ctx, file.path, uriForPath(req.Root, file.path), file.content)
		if err != nil {
			return Result{}, fmt.Errorf("read document symbols for %s: %w", filepath.ToSlash(file.path), err)
		}

		summaries = append(summaries, FileSummary{
			Path:         file.path,
			Language:     file.language,
			Bytes:        int64(len(file.content)),
			Symbols:      symbols,
			SymbolicSExp: buildSExpression(file.path, symbols),
		})
		symbolRefsByFile[file.path] = refs
		documents[file.path] = &scip.Document{
			RelativePath:     filepath.ToSlash(file.path),
			Language:         file.language,
			PositionEncoding: scip.PositionEncoding_UTF8CodeUnitOffsetFromLineStart,
		}
		if !req.SymbolicOnly {
			documents[file.path].Text = file.content
		}
		occurrences[file.path] = map[string]struct{}{}
	}

	for _, summary := range summaries {
		if summary.Skipped {
			continue
		}
		doc := documents[summary.Path]
		refs := symbolRefsByFile[summary.Path]
		for index, symbol := range summary.Symbols {
			symbolID := formatSymbol(summary.Language, symbol, index)
			doc.Symbols = append(doc.Symbols, &scip.SymbolInformation{
				Symbol:      symbolID,
				DisplayName: symbol.Name,
				Kind:        symbolKind(symbol.Kind),
			})
			// Seed each symbol with its definition before layering on reference
			// occurrences returned by the language server.
			addOccurrence(occurrences[summary.Path], doc, scip.Occurrence{
				Range:       symbolRange(symbol),
				Symbol:      symbolID,
				SymbolRoles: int32(scip.SymbolRole_Definition),
			})
			if index >= len(refs) {
				continue
			}
			locations, refErr := client.References(ctx, uriForPath(req.Root, summary.Path), refs[index].Position)
			if refErr != nil {
				key := refErr.Error()
				if _, ok := referenceWarnings[key]; !ok {
					referenceWarnings[key] = struct{}{}
					warnings = append(warnings, "reference lookup failed: "+refErr.Error())
				}
				continue
			}
			for _, location := range locations {
				rel, ok := relativePathFromURI(req.Root, location.URI)
				if !ok {
					continue
				}
				if rel == summary.Path && rangeWithinSymbol(symbol, location.Range) {
					continue
				}
				targetDoc := documents[rel]
				if targetDoc == nil {
					continue
				}
				roles := int32(scip.SymbolRole_ReadAccess)
				if rel == summary.Path && sameRange(location.Range, refs[index].Range) {
					roles = int32(scip.SymbolRole_Definition)
				}
				addOccurrence(occurrences[rel], targetDoc, scip.Occurrence{
					Range:       location.Range.toSCIP(),
					Symbol:      symbolID,
					SymbolRoles: roles,
				})
			}
		}
	}

	orderedDocs := make([]*scip.Document, 0, len(documents))
	for _, rel := range files {
		if doc := documents[rel]; doc != nil {
			orderedDocs = append(orderedDocs, doc)
		}
	}
	if err := writeSCIPSnapshot(outputPath, req.Root, "scip-cli-lsp", orderedDocs); err != nil {
		return Result{}, fmt.Errorf("write scip index: %w", err)
	}

	return Result{
		Indexer:       "scip-lsp",
		Root:          req.Root,
		Language:      firstNonEmpty(req.Language, "mixed"),
		FilesScanned:  len(files),
		FilesIndexed:  len(orderedDocs),
		FilesSkipped:  filesSkipped,
		Sampled:       sampled,
		SCIPPath:      outputPath,
		FileSummaries: summaries,
		Warnings:      warnings,
		CommandLine:   commandLine,
	}, nil
}
