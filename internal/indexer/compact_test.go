package indexer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PrepareResult", func() {
	It("builds compact dictionaries and trims inline detail", func() {
		result := PrepareResult(IndexRequest{
			MaxFiles:          1,
			MaxSymbolsPerFile: 1,
			SummaryDetail:     "normal",
			MaxTokensApprox:   4000,
			ResponseMode:      "detailed",
			IncludeDebug:      true,
		}, Result{
			Dependencies: []ModuleDependency{{Path: "example.com/lib"}},
			Warnings:     []string{"security warning: invisible Unicode character", "indexed"},
			CommandLine:  []string{"scip-go", "./..."},
			FileSummaries: []FileSummary{
				{
					Path:     "main.go",
					Language: "go",
					Symbols: []Symbol{
						{Name: "main", Kind: "function", Path: "main.go", StartLine: 1, EndLine: 3},
						{Name: "helper", Kind: "function", Path: "main.go", StartLine: 5, EndLine: 7},
					},
					SymbolicSExp: "(file \"main.go\" (sym \"function\" \"main\" 1 3))",
				},
				{
					Path:     "pkg/api.go",
					Language: "go",
					Symbols:  []Symbol{{Name: "Serve", Kind: "function", Path: "pkg/api.go", StartLine: 1, EndLine: 5}},
				},
			},
		})

		Expect(result.Budget.Applied).To(BeTrue())
		Expect(result.Budget.Truncated).To(BeTrue())
		Expect(result.FileSummaries).To(HaveLen(1))
		Expect(result.FileSummaries[0].Symbols).To(HaveLen(1))
		Expect(result.StringTables.Paths).NotTo(BeEmpty())
		Expect(result.CompactDeps).To(HaveLen(1))
		Expect(result.CompactFiles).To(HaveLen(1))
		Expect(result.CompactSymbols).To(HaveLen(1))
		Expect(result.Relationships).NotTo(BeEmpty())
		Expect(result.Packages).NotTo(BeEmpty())
		Expect(result.WarningNotices).NotTo(BeEmpty())
		Expect(result.Warnings).To(HaveLen(1))
		Expect(result.Debug).NotTo(BeNil())
		Expect(result.Debug.CommandLine).To(ContainElement("scip-go"))
		Expect(result.CommandLine).To(BeNil())
	})

	It("defaults to compact mode without inline file summaries", func() {
		result := PrepareResult(IndexRequest{
			MaxFiles:          2,
			MaxSymbolsPerFile: 2,
			SummaryDetail:     "normal",
			MaxTokensApprox:   4000,
			ResponseMode:      "compact",
		}, Result{
			FileSummaries: []FileSummary{
				{Path: "main.go", Language: "go", Symbols: []Symbol{{Name: "main", Kind: "function", Path: "main.go"}}},
			},
		})

		Expect(result.FileSummaries).To(BeNil())
		Expect(result.CompactFiles).To(HaveLen(1))
		Expect(result.CompactSymbols).To(HaveLen(1))
		Expect(result.Budget.OmittedFields).To(ContainElement("inlineFileSummaries"))
	})

	It("returns package-first summaries for larger repos", func() {
		result := PrepareResult(IndexRequest{
			MaxFiles:          2,
			MaxSymbolsPerFile: 2,
			SummaryDetail:     "normal",
			MaxTokensApprox:   4000,
			ResponseMode:      "compact",
		}, Result{
			FileSummaries: []FileSummary{
				{Path: "pkg/a.go", Language: "go", Symbols: []Symbol{{Name: "A", Kind: "function", Path: "pkg/a.go"}}},
				{Path: "pkg/b.go", Language: "go", Symbols: []Symbol{{Name: "B", Kind: "function", Path: "pkg/b.go"}}},
				{Path: "pkg/c.go", Language: "go", Symbols: []Symbol{{Name: "C", Kind: "function", Path: "pkg/c.go"}}},
				{Path: "cmd/main.go", Language: "go", Symbols: []Symbol{{Name: "main", Kind: "function", Path: "cmd/main.go"}}},
				{Path: "internal/d.go", Language: "go", Symbols: []Symbol{{Name: "D", Kind: "function", Path: "internal/d.go"}}},
			},
		})

		Expect(result.FileSummaries).To(BeNil())
		Expect(result.Packages).To(HaveLen(3))
		Expect(result.CompactSymbols).To(BeNil())
		Expect(result.Budget.OmittedFields).To(ContainElement("packageFirst"))
	})

	It("drops inline file summaries when the approximate token budget is exceeded", func() {
		result := PrepareResult(IndexRequest{
			MaxFiles:          2,
			MaxSymbolsPerFile: 4,
			SummaryDetail:     "deep",
			MaxTokensApprox:   1,
			IncludeSpans:      true,
		}, Result{
			FileSummaries: []FileSummary{
				{
					Path:         "main.go",
					Language:     "go",
					Symbols:      []Symbol{{Name: "main", Kind: "function", Path: "main.go", StartLine: 1, EndLine: 3}},
					SymbolicSExp: "(file \"main.go\")",
				},
			},
		})

		Expect(result.FileSummaries).To(BeNil())
		Expect(result.Budget.Truncated).To(BeTrue())
		Expect(result.Budget.OmittedFields).To(ContainElement("inlineFileSummaries"))
	})

	It("auto-downgrades compact mode to handles for large results", func() {
		auto := true
		result := PrepareResult(IndexRequest{
			MaxFiles:                  200,
			MaxSymbolsPerFile:         4,
			SummaryDetail:             "normal",
			MaxTokensApprox:           64000,
			ResponseMode:              "compact",
			AutoResponseMode:          &auto,
			AutoHandleFileThreshold:   2,
			AutoHandleSymbolThreshold: 3000,
		}, Result{
			FilesIndexed: 3,
			FileSummaries: []FileSummary{
				{Path: "a.go", Language: "go", Symbols: []Symbol{{Name: "A", Kind: "function", Path: "a.go"}}},
				{Path: "b.go", Language: "go", Symbols: []Symbol{{Name: "B", Kind: "function", Path: "b.go"}}},
				{Path: "c.go", Language: "go", Symbols: []Symbol{{Name: "C", Kind: "function", Path: "c.go"}}},
			},
		})

		Expect(result.Budget.ResponseMode).To(Equal("handles"))
		Expect(result.CompactSymbols).To(BeNil())
		Expect(result.Budget.OmittedFields).To(ContainElement("autoHandles"))
	})

	It("keeps detailed mode when explicitly requested even for large inputs", func() {
		auto := true
		result := PrepareResult(IndexRequest{
			MaxFiles:                  200,
			MaxSymbolsPerFile:         4,
			SummaryDetail:             "normal",
			MaxTokensApprox:           64000,
			ResponseMode:              "detailed",
			AutoResponseMode:          &auto,
			AutoHandleFileThreshold:   1,
			AutoHandleSymbolThreshold: 1,
		}, Result{
			FilesIndexed: 3,
			FileSummaries: []FileSummary{
				{Path: "a.go", Language: "go", Symbols: []Symbol{{Name: "A", Kind: "function", Path: "a.go"}}},
				{Path: "b.go", Language: "go", Symbols: []Symbol{{Name: "B", Kind: "function", Path: "b.go"}}},
			},
		})

		Expect(result.Budget.ResponseMode).To(Equal("detailed"))
		Expect(result.FileSummaries).NotTo(BeNil())
	})

	It("falls back through budget ladder and preserves compact file handles", func() {
		result := PrepareResult(IndexRequest{
			MaxFiles:          6,
			MaxSymbolsPerFile: 6,
			SummaryDetail:     "deep",
			MaxTokensApprox:   1,
			ResponseMode:      "detailed",
		}, Result{
			FilesIndexed: 6,
			Warnings: []string{
				"security warning: invisible unicode",
				"dependency scan failed: boom",
				"indexed",
			},
			FileSummaries: []FileSummary{
				{Path: "pkg/a.go", Language: "go", SymbolicSExp: "(file a)", Symbols: []Symbol{{Name: "A", Kind: "function", Path: "pkg/a.go"}}},
				{Path: "pkg/b.go", Language: "go", SymbolicSExp: "(file b)", Symbols: []Symbol{{Name: "B", Kind: "function", Path: "pkg/b.go"}}},
				{Path: "pkg/c.go", Language: "go", SymbolicSExp: "(file c)", Symbols: []Symbol{{Name: "C", Kind: "function", Path: "pkg/c.go"}}},
			},
		})

		Expect(result.Budget.Truncated).To(BeTrue())
		Expect(result.CompactFiles).NotTo(BeEmpty())
		Expect(result.CompactFiles[0].HandleID).NotTo(BeEmpty())
		Expect(result.WarningNotices).NotTo(BeEmpty())
	})

	It("normalizes path keys before adding them to string tables", func() {
		result := PrepareResult(IndexRequest{
			MaxFiles:          4,
			MaxSymbolsPerFile: 4,
			SummaryDetail:     "normal",
			MaxTokensApprox:   64000,
			ResponseMode:      "detailed",
		}, Result{
			FileSummaries: []FileSummary{
				{Path: "pkg/./file.go", Language: "go", Symbols: []Symbol{{Name: "A", Kind: "function", Path: "pkg/./file.go"}}},
				{Path: "pkg/file.go", Language: "go", Symbols: []Symbol{{Name: "B", Kind: "function", Path: "pkg/file.go"}}},
			},
		})

		values := map[string]struct{}{}
		for _, entry := range result.StringTables.Paths {
			values[entry.Value] = struct{}{}
		}
		Expect(values).To(HaveKey("pkg/file.go"))
	})
})
