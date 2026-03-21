package indexer

import "testing"

func TestPrepareResultWrapper(t *testing.T) {
	result := PrepareResult(IndexRequest{
		MaxFiles:          1,
		MaxSymbolsPerFile: 1,
		SummaryDetail:     "normal",
		MaxTokensApprox:   4000,
		ResponseMode:      "compact",
	}, Result{
		FileSummaries: []FileSummary{
			{Path: "main.go", Language: "go", Symbols: []Symbol{{Name: "main", Kind: "function", Path: "main.go"}}},
		},
	})
	if len(result.CompactFiles) != 1 {
		t.Fatalf("expected top-level wrapper to delegate to compact package")
	}
}
