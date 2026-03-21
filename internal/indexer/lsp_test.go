package indexer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	scip "github.com/sourcegraph/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"
)

type stubLSPClient struct {
	symbols map[string][]Symbol
	refs    map[string][]lspSymbolRef
	uses    map[string][]lspLocation
}

func (s *stubLSPClient) Initialize(context.Context, string, map[string]any) error {
	return nil
}

func (s *stubLSPClient) OpenDocument(context.Context, string, string, string) error {
	return nil
}

func (s *stubLSPClient) DocumentSymbols(_ context.Context, _ string, uri, _ string) ([]Symbol, []lspSymbolRef, error) {
	return append([]Symbol(nil), s.symbols[uri]...), append([]lspSymbolRef(nil), s.refs[uri]...), nil
}

func (s *stubLSPClient) References(_ context.Context, uri string, position lspPosition) ([]lspLocation, error) {
	key := fmt.Sprintf("%s:%d:%d", uri, position.Line, position.Character)
	return append([]lspLocation(nil), s.uses[key]...), nil
}

func (s *stubLSPClient) Close(context.Context) error {
	return nil
}

var _ = Describe("LSPIndexer", func() {
	It("indexes through an injected lsp client", func() {
		tempRoot := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(tempRoot, "app.py"), []byte("def greet():\n    return 'hi'\n\nprint(greet())\n"), 0o644)).To(Succeed())

		uri := uriForPath(tempRoot, "app.py")
		client := &stubLSPClient{
			symbols: map[string][]Symbol{
				uri: {{
					Name:      "greet",
					Kind:      "function",
					Path:      "app.py",
					StartLine: 1,
					EndLine:   2,
				}},
			},
			refs: map[string][]lspSymbolRef{
				uri: {{
					Name:     "greet",
					Kind:     "function",
					Range:    lspRange{Start: lspPosition{Line: 0, Character: 4}, End: lspPosition{Line: 1, Character: 0}},
					Position: lspPosition{Line: 0, Character: 4},
				}},
			},
			uses: map[string][]lspLocation{
				fmt.Sprintf("%s:%d:%d", uri, 0, 4): {
					{URI: uri, Range: lspRange{Start: lspPosition{Line: 0, Character: 4}, End: lspPosition{Line: 1, Character: 0}}},
					{URI: uri, Range: lspRange{Start: lspPosition{Line: 3, Character: 6}, End: lspPosition{Line: 3, Character: 11}}},
				},
			},
		}

		indexer := &LSPIndexer{
			newClient: func(context.Context, IndexRequest) (lspClient, []string, error) {
				return client, []string{"stub-lsp"}, nil
			},
		}

		result, err := indexer.Index(context.Background(), IndexRequest{
			Root:       tempRoot,
			Language:   "python",
			Indexer:    "scip-lsp",
			LSPCommand: "stub-lsp",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Indexer).To(Equal("scip-lsp"))
		Expect(result.FileSummaries).To(HaveLen(1))
		Expect(result.FileSummaries[0].Symbols).To(HaveLen(1))
		Expect(result.CommandLine).To(Equal([]string{"stub-lsp"}))

		payload, err := os.ReadFile(result.SCIPPath)
		Expect(err).NotTo(HaveOccurred())

		var index scip.Index
		Expect(proto.Unmarshal(payload, &index)).To(Succeed())
		Expect(index.Documents).To(HaveLen(1))
		Expect(index.Documents[0].Occurrences).To(HaveLen(2))
	})

	It("indexes through the stdio lsp transport", func() {
		tempRoot := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(tempRoot, "main.zig"), []byte("fn greet(name: []const u8) usize {\n    return name.len;\n}\n\npub fn main() void {\n    _ = greet(\"zig\");\n}\n"), 0o644)).To(Succeed())

		result, err := Dispatch(context.Background(), IndexRequest{
			Root:       tempRoot,
			Language:   "zig",
			Indexer:    "scip-lsp",
			LSPCommand: os.Args[0],
			LSPArgs:    []string{"-test.run=TestLSPHelperProcess"},
			LSPEnv:     []string{"GO_WANT_LSP_HELPER=1"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Indexer).To(Equal("scip-lsp"))
		Expect(result.FileSummaries).To(HaveLen(1))
		Expect(result.FileSummaries[0].Symbols).NotTo(BeEmpty())
		Expect(result.FileSummaries[0].Symbols[0].Signature).To(ContainSubstring("fn greet(name: []const u8) usize"))
		Expect(result.FileSummaries[0].Symbols[0].BodyText).To(ContainSubstring("return name.len;"))

		payload, err := os.ReadFile(result.SCIPPath)
		Expect(err).NotTo(HaveOccurred())

		var index scip.Index
		Expect(proto.Unmarshal(payload, &index)).To(Succeed())
		Expect(index.Documents).To(HaveLen(1))
		Expect(index.Documents[0].Symbols).NotTo(BeEmpty())
		Expect(index.Documents[0].Occurrences).To(HaveLen(2))
	})
})

func TestLSPHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_LSP_HELPER") != "1" {
		return
	}

	runFakeLSPServer(os.Stdin, os.Stdout)
	os.Exit(0)
}

func runFakeLSPServer(reader io.Reader, writer io.Writer) {
	buffered := bufio.NewReader(reader)
	currentURI := ""
	for {
		body, err := readPacket(buffered)
		if err != nil {
			return
		}

		var request struct {
			ID     int64           `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			continue
		}

		switch request.Method {
		case "initialize":
			writeFakeLSPResponse(writer, request.ID, map[string]any{
				"capabilities": map[string]any{
					"documentSymbolProvider": true,
					"referencesProvider":     true,
				},
			})
		case "textDocument/didOpen":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			if err := json.Unmarshal(request.Params, &params); err == nil {
				currentURI = params.TextDocument.URI
			}
		case "textDocument/documentSymbol":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			if err := json.Unmarshal(request.Params, &params); err == nil {
				currentURI = params.TextDocument.URI
			}
			writeFakeLSPResponse(writer, request.ID, []map[string]any{
				{
					"name": "greet",
					"kind": 12,
					"range": map[string]any{
						"start": map[string]int{"line": 0, "character": 0},
						"end":   map[string]int{"line": 2, "character": 1},
					},
					"selectionRange": map[string]any{
						"start": map[string]int{"line": 0, "character": 3},
						"end":   map[string]int{"line": 0, "character": 8},
					},
				},
			})
		case "textDocument/references":
			writeFakeLSPResponse(writer, request.ID, []map[string]any{
				{
					"uri": currentURI,
					"range": map[string]any{
						"start": map[string]int{"line": 0, "character": 3},
						"end":   map[string]int{"line": 0, "character": 8},
					},
				},
				{
					"uri": currentURI,
					"range": map[string]any{
						"start": map[string]int{"line": 5, "character": 8},
						"end":   map[string]int{"line": 5, "character": 13},
					},
				},
			})
		case "shutdown":
			writeFakeLSPResponse(writer, request.ID, map[string]any{})
		case "exit":
			return
		}
	}
}

func writeFakeLSPResponse(writer io.Writer, id int64, result any) {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
}
