package indexer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Query helpers", func() {
	var result Result

	BeforeEach(func() {
		// This hand-built result keeps the query helper specs deterministic and
		// avoids coupling them to the heavier indexer integration fixtures.
		result = Result{
			Root: "/workspace",
			FileSummaries: []FileSummary{
				{
					Path:     "cmd/scip-cli/main.go",
					Language: "go",
					Bytes:    120,
					Symbols: []Symbol{
						{Name: "runCLI", Kind: "function", Path: "cmd/scip-cli/main.go", StartLine: 10, EndLine: 40},
					},
				},
				{
					Path:     "internal/indexer/query.go",
					Language: "go",
					Bytes:    220,
					Symbols: []Symbol{
						{Name: "SearchFiles", Kind: "function", Path: "internal/indexer/query.go", StartLine: 12, EndLine: 30},
						{Name: "FindPackage", Kind: "function", Path: "internal/indexer/query.go", StartLine: 50, EndLine: 80},
					},
				},
			},
			Warnings: []string{
				"security warning: invisible Unicode character in internal/indexer/query.go",
				"tree-sitter fallback in use",
			},
			Dependencies: []ModuleDependency{
				{Path: "example.com/dep"},
			},
		}
	})

	It("finds files by path and symbol content", func() {
		matches := SearchFiles(result, "query")
		Expect(matches).To(HaveLen(1))
		Expect(matches[0].File.Path).To(Equal("internal/indexer/query.go"))
		Expect(matches[0].Package).To(Equal("internal/indexer"))
	})

	It("finds symbols across files", func() {
		matches := SearchSymbols(result, "find")
		Expect(matches).To(HaveLen(1))
		Expect(matches[0].Name).To(Equal("FindPackage"))
	})

	It("finds matching warnings", func() {
		matches := SearchWarnings(result, "unicode")
		Expect(matches).To(HaveLen(1))
		Expect(matches[0]).To(ContainSubstring("security warning"))
	})

	It("locates a file by exact path", func() {
		// A leading "./" should normalize to the same relative path stored in the result.
		match, ok := FindFile(result, "./cmd/scip-cli/main.go")
		Expect(ok).To(BeTrue())
		Expect(match.File.Path).To(Equal("cmd/scip-cli/main.go"))
	})

	It("locates a package by exact name", func() {
		match, ok := FindPackage(result, "internal/indexer")
		Expect(ok).To(BeTrue())
		Expect(match.Package.FileCount).To(Equal(1))
		Expect(match.Files[0].Path).To(Equal("internal/indexer/query.go"))
	})
})
