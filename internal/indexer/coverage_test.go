package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type stubIndexCommand struct {
	name      string
	supported bool
	result    Result
	err       error
}

func (command stubIndexCommand) Name() string {
	return command.name
}

func (command stubIndexCommand) Supports(string, IndexRequest) bool {
	return command.supported
}

func (command stubIndexCommand) Execute(context.Context, IndexRequest) (Result, error) {
	return command.result, command.err
}

type configurableLSPClient struct {
	initializeErr error
	openErr       error
	documentErr   error
	referenceErr  error
	closeErr      error
}

func (client *configurableLSPClient) Initialize(context.Context, string, map[string]any) error {
	return client.initializeErr
}

func (client *configurableLSPClient) OpenDocument(context.Context, string, string, string) error {
	return client.openErr
}

func (client *configurableLSPClient) DocumentSymbols(context.Context, string, string, string) ([]Symbol, []lspSymbolRef, error) {
	if client.documentErr != nil {
		return nil, nil, client.documentErr
	}
	return []Symbol{{Name: "greet", Kind: "function", Path: "app.py", StartLine: 1, EndLine: 1}}, []lspSymbolRef{{
		Name:     "greet",
		Kind:     "function",
		Range:    lspRange{Start: lspPosition{Line: 0, Character: 0}, End: lspPosition{Line: 0, Character: 5}},
		Position: lspPosition{Line: 0, Character: 0},
	}}, nil
}

func (client *configurableLSPClient) References(context.Context, string, lspPosition) ([]lspLocation, error) {
	if client.referenceErr != nil {
		return nil, client.referenceErr
	}
	return nil, nil
}

func (client *configurableLSPClient) Close(context.Context) error {
	return client.closeErr
}

var _ = Describe("Additional coverage helpers", func() {
	It("covers command dispatch selection branches", func() {
		original := append([]IndexCommand(nil), commandRegistry...)
		commandRegistry = nil
		DeferCleanup(func() {
			commandRegistry = original
		})

		commandRegistry = []IndexCommand{
			stubIndexCommand{name: "tree-sitter", supported: true},
			stubIndexCommand{name: "symbolic-fallback", supported: true},
			stubIndexCommand{name: "go", supported: true, result: Result{Indexer: "go"}},
			stubIndexCommand{name: "scip-lsp", supported: true, result: Result{Indexer: "scip-lsp"}},
			stubIndexCommand{name: "custom", supported: true, result: Result{Indexer: "custom"}},
		}

		command, err := commandForRequest(IndexRequest{Root: ".", Language: "go"})
		Expect(err).NotTo(HaveOccurred())
		Expect(command.Name()).To(Equal("go"))

		command, err = commandForRequest(IndexRequest{Root: ".", Language: "python", LSPCommand: "pylsp"})
		Expect(err).NotTo(HaveOccurred())
		Expect(command.Name()).To(Equal("scip-lsp"))

		commandRegistry = []IndexCommand{
			stubIndexCommand{name: "tree-sitter", supported: true},
			stubIndexCommand{name: "symbolic-fallback", supported: true},
			stubIndexCommand{name: "go", supported: false, result: Result{Indexer: "go"}},
			stubIndexCommand{name: "custom", supported: true, result: Result{Indexer: "custom"}},
		}
		command, err = commandForRequest(IndexRequest{Root: ".", Language: "python"})
		Expect(err).NotTo(HaveOccurred())
		Expect(command.Name()).To(Equal("custom"))

		commandRegistry = []IndexCommand{stubIndexCommand{name: "symbolic-fallback", supported: true}}
		command, err = commandForRequest(IndexRequest{Root: ".", Language: "python"})
		Expect(err).NotTo(HaveOccurred())
		Expect(command.Name()).To(Equal("symbolic-fallback"))

		commandRegistry = nil
		_, err = commandForRequest(IndexRequest{Root: "."})
		Expect(err).To(MatchError("no index commands have been registered"))

		commandRegistry = []IndexCommand{stubIndexCommand{name: "go", supported: false}}
		_, err = commandForRequest(IndexRequest{Root: ".", Indexer: "missing"})
		Expect(err).To(MatchError(ContainSubstring("matches indexer")))
		_, err = commandForRequest(IndexRequest{Root: ".", Language: "python"})
		Expect(err).To(MatchError(ContainSubstring("supports language")))
		_, err = commandForRequest(IndexRequest{Root: "."})
		Expect(err).To(MatchError("no registered index command supports the request"))

		commandRegistry = []IndexCommand{stubIndexCommand{name: "go", supported: true, result: Result{Indexer: "go"}}}
		result, err := Dispatch(context.Background(), IndexRequest{Root: ".", Language: "go"})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Indexer).To(Equal("go"))

		Expect(pickLanguage(IndexRequest{Language: "python"})).To(Equal("python"))
		Expect(pickLanguage(IndexRequest{Paths: []string{"file.go"}})).To(Equal("go"))
		Expect(pickLanguage(IndexRequest{Paths: []string{"a.go", "b.go"}})).To(BeEmpty())
	})

	It("covers query helpers edge cases and sorting", func() {
		result := Result{
			FileSummaries: []FileSummary{
				{Path: "b/main.go", Language: "go", Symbols: []Symbol{{Name: "Run", Kind: "function", Path: "b/main.go", StartLine: 4}}},
				{Path: "a/main.go", Language: "go", Symbols: []Symbol{{Name: "Run", Kind: "type", Path: "a/main.go", StartLine: 1}}},
			},
			Warnings: []string{"warning A", "warning B"},
			Dependencies: []ModuleDependency{
				{Path: "dep1"}, {Path: "dep2"}, {Path: "dep3"}, {Path: "dep4"}, {Path: "dep5"}, {Path: "dep6"}, {Path: "dep7"},
			},
		}

		Expect(SearchFiles(result, "  ")).To(BeNil())
		Expect(SearchSymbols(result, "")).To(BeNil())
		Expect(SearchWarnings(result, "")).To(Equal([]string{"warning A", "warning B"}))
		Expect(SearchSymbols(result, "run")[0].Path).To(Equal("a/main.go"))
		Expect(SearchFiles(result, "go")[0].File.Path).To(Equal("a/main.go"))

		_, ok := FindFile(result, " ")
		Expect(ok).To(BeFalse())
		_, ok = FindPackage(result, " ")
		Expect(ok).To(BeFalse())

		match, ok := FindPackage(result, "a")
		Expect(ok).To(BeTrue())
		Expect(match.Package.Dependencies).To(HaveLen(6))
		Expect(fileMatchesQuery(result.FileSummaries[0], "function")).To(BeTrue())
		Expect(fileMatchesQuery(result.FileSummaries[0], "missing")).To(BeFalse())
		Expect(normalizeQuery(" HeLLo ")).To(Equal("hello"))
		Expect(matchesQuery("Hello", "ell")).To(BeTrue())
		Expect(normalizePath("./a/../b/main.go")).To(Equal("b/main.go"))
	})

	It("covers lsp symbol decoding helpers", func() {
		documentPayload, err := json.Marshal([]map[string]any{{
			"name":   "Parent",
			"detail": "",
			"kind":   5,
			"range": map[string]any{
				"start": map[string]int{"line": 0, "character": 0},
				"end":   map[string]int{"line": 2, "character": 0},
			},
			"selectionRange": map[string]any{
				"start": map[string]int{"line": 0, "character": 4},
				"end":   map[string]int{"line": 0, "character": 10},
			},
			"children": []map[string]any{{
				"name":   "child",
				"detail": "child()",
				"kind":   13,
				"range": map[string]any{
					"start": map[string]int{"line": 1, "character": 2},
					"end":   map[string]int{"line": 1, "character": 8},
				},
				"selectionRange": map[string]any{},
				"children":       []map[string]any{},
			}},
		}})
		Expect(err).NotTo(HaveOccurred())

		symbols, refs, err := decodeDocumentSymbols("app.ts", "class Parent {}\nconst child = 1\n", documentPayload)
		Expect(err).NotTo(HaveOccurred())
		Expect(symbols).To(HaveLen(2))
		Expect(refs).To(HaveLen(2))
		Expect(symbols[0].Kind).To(Equal("type"))
		Expect(symbols[1].Kind).To(Equal("variable"))

		flatPayload, err := json.Marshal([]map[string]any{{
			"name":   "EnumValue",
			"detail": "",
			"kind":   10,
			"location": map[string]any{
				"uri": "file:///tmp/app.ts",
				"range": map[string]any{
					"start": map[string]int{"line": 3, "character": 0},
					"end":   map[string]int{"line": 3, "character": 5},
				},
			},
		}})
		Expect(err).NotTo(HaveOccurred())

		symbols, refs, err = decodeDocumentSymbols("app.ts", "line0\nline1\nline2\nvalue\n", flatPayload)
		Expect(err).NotTo(HaveOccurred())
		Expect(symbols[0].Kind).To(Equal("enum"))
		Expect(refs[0].Position.Line).To(Equal(3))
		symbols, refs, err = decodeDocumentSymbols("app.ts", "line0\r\nline1\r\nline2\r\nvalue\r\n", flatPayload)
		Expect(err).NotTo(HaveOccurred())
		Expect(symbols[0].Signature).To(Equal("value"))
		Expect(symbols[0].BodyText).To(Equal("value"))
		Expect(refs[0].Position.Line).To(Equal(3))
		Expect(symbolSignature("", []string{"hello"}, 0)).To(Equal("hello"))
		Expect(symbolSignature("  detail  ", []string{"ignored"}, 0)).To(Equal("detail"))
		Expect(symbolSignature("", []string{"ignored"}, 9)).To(BeEmpty())
		Expect(symbolBody(nil, lspRange{})).To(BeEmpty())
		Expect(symbolBody([]string{"a", "b"}, lspRange{Start: lspPosition{Line: 3}, End: lspPosition{Line: 4}})).To(BeEmpty())
		Expect(symbolBody([]string{"a", "b", "c"}, lspRange{Start: lspPosition{Line: 0}, End: lspPosition{Line: 99}})).To(Equal("a\nb\nc"))
		Expect(splitSourceLines("a\r\nb\r\n")).To(Equal([]string{"a", "b"}))
		Expect(normalizeLSPKind(22)).To(Equal("type"))
		Expect(normalizeLSPKind(23)).To(Equal("enum"))
		Expect(normalizeLSPKind(14)).To(Equal("variable"))
		Expect(normalizeLSPKind(99)).To(Equal("function"))
	})

	It("covers lsp indexer failure and warning paths", func() {
		tempRoot := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(tempRoot, "app.py"), []byte("print('hi')\n"), 0o644)).To(Succeed())

		indexer := NewLSPIndexer()
		Expect(indexer.newClient).NotTo(BeNil())
		Expect((lspIndexCommand{}).Name()).To(Equal("scip-lsp"))
		Expect((lspIndexCommand{}).Supports("", IndexRequest{})).To(BeFalse())
		Expect((lspIndexCommand{}).Supports("", IndexRequest{LSPCommand: "pylsp"})).To(BeTrue())

		_, err := (&LSPIndexer{}).Index(context.Background(), IndexRequest{Root: tempRoot, Indexer: "scip-lsp"})
		Expect(err).To(MatchError("lspCommand is required when using the scip-lsp indexer"))

		_, err = (&LSPIndexer{
			newClient: func(context.Context, IndexRequest) (lspClient, []string, error) {
				return nil, nil, errors.New("connect failed")
			},
		}).Index(context.Background(), IndexRequest{Root: tempRoot, Language: "python", Indexer: "scip-lsp", LSPCommand: "pylsp"})
		Expect(err).To(MatchError("connect failed"))

		_, err = (&LSPIndexer{
			newClient: func(context.Context, IndexRequest) (lspClient, []string, error) {
				return &configurableLSPClient{initializeErr: errors.New("init failed")}, []string{"pylsp"}, nil
			},
		}).Index(context.Background(), IndexRequest{Root: tempRoot, Language: "python", Indexer: "scip-lsp", LSPCommand: "pylsp"})
		Expect(err).To(MatchError(ContainSubstring("initialize lsp client")))

		_, err = (&LSPIndexer{
			newClient: func(context.Context, IndexRequest) (lspClient, []string, error) {
				return &configurableLSPClient{openErr: errors.New("open failed")}, []string{"pylsp"}, nil
			},
		}).Index(context.Background(), IndexRequest{Root: tempRoot, Language: "python", Indexer: "scip-lsp", LSPCommand: "pylsp"})
		Expect(err).To(MatchError(ContainSubstring("open app.py in lsp")))

		_, err = (&LSPIndexer{
			newClient: func(context.Context, IndexRequest) (lspClient, []string, error) {
				return &configurableLSPClient{documentErr: errors.New("symbols failed")}, []string{"pylsp"}, nil
			},
		}).Index(context.Background(), IndexRequest{Root: tempRoot, Language: "python", Indexer: "scip-lsp", LSPCommand: "pylsp"})
		Expect(err).To(MatchError(ContainSubstring("read document symbols")))

		result, err := (&LSPIndexer{
			newClient: func(context.Context, IndexRequest) (lspClient, []string, error) {
				return &configurableLSPClient{referenceErr: errors.New("refs failed")}, []string{"pylsp"}, nil
			},
		}).Index(context.Background(), IndexRequest{Root: tempRoot, Language: "python", Indexer: "scip-lsp", LSPCommand: "pylsp"})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Warnings).To(ContainElement(ContainSubstring("reference lookup failed")))
	})
})
