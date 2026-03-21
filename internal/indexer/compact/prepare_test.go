package compact

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/vjanelle/scip-cli/internal/indexer/model"
)

var _ = Describe("PrepareResult", func() {
	It("builds compact structures and trims inline detail", func() {
		result := PrepareResult(model.IndexRequest{
			MaxFiles:          1,
			MaxSymbolsPerFile: 1,
			SummaryDetail:     "normal",
			MaxTokensApprox:   4000,
			ResponseMode:      "detailed",
			IncludeDebug:      true,
		}, model.Result{
			Dependencies: []model.ModuleDependency{{Path: "example.com/lib"}},
			Warnings:     []string{"security warning: invisible Unicode character", "indexed"},
			CommandLine:  []string{"scip-go", "./..."},
			FileSummaries: []model.FileSummary{
				{
					Path:     "main.go",
					Language: "go",
					Symbols: []model.Symbol{
						{Name: "main", Kind: "function", Path: "main.go", StartLine: 1, EndLine: 3},
						{Name: "helper", Kind: "function", Path: "main.go", StartLine: 5, EndLine: 7},
					},
					SymbolicSExp: "(file \"main.go\" (sym \"function\" \"main\" 1 3))",
				},
				{
					Path:     "pkg/api.go",
					Language: "go",
					Symbols:  []model.Symbol{{Name: "Serve", Kind: "function", Path: "pkg/api.go", StartLine: 1, EndLine: 5}},
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

	It("handles response policy modes", func() {
		auto := true
		result := PrepareResult(model.IndexRequest{
			MaxFiles:                  200,
			MaxSymbolsPerFile:         4,
			SummaryDetail:             "normal",
			MaxTokensApprox:           64000,
			ResponseMode:              "compact",
			AutoResponseMode:          &auto,
			AutoHandleFileThreshold:   2,
			AutoHandleSymbolThreshold: 3000,
		}, model.Result{
			FilesIndexed: 3,
			FileSummaries: []model.FileSummary{
				{Path: "a.go", Language: "go", Symbols: []model.Symbol{{Name: "A", Kind: "function", Path: "a.go"}}},
				{Path: "b.go", Language: "go", Symbols: []model.Symbol{{Name: "B", Kind: "function", Path: "b.go"}}},
				{Path: "c.go", Language: "go", Symbols: []model.Symbol{{Name: "C", Kind: "function", Path: "c.go"}}},
			},
		})
		Expect(result.Budget.ResponseMode).To(Equal("handles"))
		Expect(result.CompactSymbols).To(BeNil())

		detailed := PrepareResult(model.IndexRequest{
			MaxFiles:                200,
			MaxSymbolsPerFile:       4,
			SummaryDetail:           "normal",
			MaxTokensApprox:         64000,
			ResponseMode:            "detailed",
			AutoResponseMode:        &auto,
			AutoHandleFileThreshold: 1,
		}, model.Result{
			FilesIndexed: 2,
			FileSummaries: []model.FileSummary{
				{Path: "a.go", Language: "go", Symbols: []model.Symbol{{Name: "A", Kind: "function", Path: "a.go"}}},
				{Path: "b.go", Language: "go", Symbols: []model.Symbol{{Name: "B", Kind: "function", Path: "b.go"}}},
			},
		})
		Expect(detailed.Budget.ResponseMode).To(Equal("detailed"))
		Expect(detailed.FileSummaries).NotTo(BeNil())
	})

	It("falls back for token budgets and normalizes path keys", func() {
		fallback := PrepareResult(model.IndexRequest{
			MaxFiles:          6,
			MaxSymbolsPerFile: 6,
			SummaryDetail:     "deep",
			MaxTokensApprox:   1,
			ResponseMode:      "detailed",
		}, model.Result{
			FilesIndexed: 6,
			Warnings: []string{
				"security warning: invisible unicode",
				"dependency scan failed: boom",
				"indexed",
			},
			FileSummaries: []model.FileSummary{
				{Path: "pkg/a.go", Language: "go", SymbolicSExp: "(file a)", Symbols: []model.Symbol{{Name: "A", Kind: "function", Path: "pkg/a.go"}}},
				{Path: "pkg/b.go", Language: "go", SymbolicSExp: "(file b)", Symbols: []model.Symbol{{Name: "B", Kind: "function", Path: "pkg/b.go"}}},
				{Path: "pkg/c.go", Language: "go", SymbolicSExp: "(file c)", Symbols: []model.Symbol{{Name: "C", Kind: "function", Path: "pkg/c.go"}}},
			},
		})
		Expect(fallback.Budget.Truncated).To(BeTrue())
		Expect(fallback.CompactFiles).NotTo(BeEmpty())
		Expect(fallback.CompactFiles[0].HandleID).NotTo(BeEmpty())
		Expect(fallback.WarningNotices).NotTo(BeEmpty())

		pathNormalized := PrepareResult(model.IndexRequest{
			MaxFiles:          4,
			MaxSymbolsPerFile: 4,
			SummaryDetail:     "normal",
			MaxTokensApprox:   64000,
			ResponseMode:      "detailed",
		}, model.Result{
			FileSummaries: []model.FileSummary{
				{Path: "pkg/./file.go", Language: "go", Symbols: []model.Symbol{{Name: "A", Kind: "function", Path: "pkg/./file.go"}}},
				{Path: "pkg/file.go", Language: "go", Symbols: []model.Symbol{{Name: "B", Kind: "function", Path: "pkg/file.go"}}},
			},
		})
		values := map[string]struct{}{}
		for _, entry := range pathNormalized.StringTables.Paths {
			values[entry.Value] = struct{}{}
		}
		Expect(values).To(HaveKey("pkg/file.go"))
	})
})

var _ = Describe("Policy Helpers", func() {
	It("covers compact policy and cloning helpers", func() {
		auto := true
		base := model.Result{
			FilesIndexed:   3,
			CompactSymbols: []model.CompactSymbol{{ID: 1}, {ID: 2}},
			CompactFiles:   []model.CompactFileSummary{{HandleID: "file:a", SymbolRefs: []int{1}, SExpID: 1}},
			Relationships:  []model.Relationship{{From: "file:a", To: "symbol:1", Kind: "contains"}, {From: "root", To: "file:a", Kind: "indexes"}},
			Warnings:       []string{"security warning: x"},
			WarningNotices: []model.WarningNotice{{Code: "security"}},
			FileSummaries:  []model.FileSummary{{Path: "a.go"}},
			StringTables:   model.StringTables{Misc: []model.StringTableEntry{{ID: 1, Value: "x"}}},
			CompactDeps:    []model.CompactDependency{{PathID: 1}},
			FullWarnings:   []string{"security warning: x"},
			Debug:          &model.DebugInfo{CommandLine: []string{"cmd"}},
		}

		mode, downgraded := effectiveResponseMode(model.IndexRequest{
			ResponseMode:            "compact",
			AutoResponseMode:        &auto,
			AutoHandleFileThreshold: 2,
		}, base)
		Expect(mode).To(Equal("handles"))
		Expect(downgraded).To(BeTrue())
		disabled := false
		Expect(autoResponseModeEnabled(model.IndexRequest{AutoResponseMode: &disabled})).To(BeFalse())

		handles := applyResponseMode(base, "handles", false)
		Expect(handles.FileSummaries).To(BeNil())
		Expect(handles.CompactSymbols).To(BeNil())
		Expect(handles.Relationships).To(BeNil())

		compactResult := applyResponseMode(base, "compact", true)
		Expect(compactResult.FileSummaries).To(BeNil())
		Expect(compactResult.CompactSymbols).To(BeNil())

		budget := model.ResultBudget{}
		applyModeOmissions(&budget, "handles", true)
		Expect(budget.OmittedFields).NotTo(BeEmpty())

		cloned := cloneResult(base)
		cloned.StringTables.Misc[0].Value = "changed"
		Expect(base.StringTables.Misc[0].Value).NotTo(Equal("changed"))

		filtered := dropSymbolRelationships(base.Relationships)
		Expect(filtered).To(HaveLen(1))
		Expect(filtered[0].To).To(Equal("file:a"))

		fallback := applyBudgetFallback(base, base, model.IndexRequest{
			MaxTokensApprox: 1,
		}, "compact", false)
		Expect(fallback.CompactFiles).NotTo(BeEmpty())
		Expect(fallback.CompactSymbols).To(BeNil())
	})
})
