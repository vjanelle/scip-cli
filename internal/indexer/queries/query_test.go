package queries

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/vjanelle/scip-cli/internal/indexer/model"
)

func testResult() model.Result {
	return model.Result{
		Root: "/workspace",
		FileSummaries: []model.FileSummary{
			{
				Path:     "cmd/scip-cli/main.go",
				Language: "go",
				Bytes:    120,
				Symbols: []model.Symbol{
					{Name: "runCLI", Kind: "function", Path: "cmd/scip-cli/main.go", StartLine: 10, EndLine: 40},
				},
			},
			{
				Path:     "internal/indexer/query.go",
				Language: "go",
				Bytes:    220,
				Symbols: []model.Symbol{
					{Name: "SearchFiles", Kind: "function", Path: "internal/indexer/query.go", StartLine: 12, EndLine: 30},
					{Name: "FindPackage", Kind: "function", Path: "internal/indexer/query.go", StartLine: 50, EndLine: 80},
				},
			},
		},
		Warnings: []string{
			"security warning: invisible Unicode character in internal/indexer/query.go",
			"tree-sitter fallback in use",
		},
		Dependencies: []model.ModuleDependency{{Path: "example.com/dep"}},
	}
}

var _ = Describe("Query Helpers", func() {
	It("finds files, symbols, warnings, and exact matches", func() {
		result := testResult()

		files := SearchFiles(result, "query")
		Expect(files).To(HaveLen(1))
		Expect(files[0].File.Path).To(Equal("internal/indexer/query.go"))
		Expect(files[0].Package).To(Equal("internal/indexer"))

		symbols := SearchSymbols(result, "find")
		Expect(symbols).To(HaveLen(1))
		Expect(symbols[0].Name).To(Equal("FindPackage"))

		warnings := SearchWarnings(result, "unicode")
		Expect(warnings).To(HaveLen(1))

		fileMatch, ok := FindFile(result, "./cmd/scip-cli/main.go")
		Expect(ok).To(BeTrue())
		Expect(fileMatch.File.Path).To(Equal("cmd/scip-cli/main.go"))

		pkgMatch, ok := FindPackage(result, "internal/indexer")
		Expect(ok).To(BeTrue())
		Expect(pkgMatch.Package.FileCount).To(Equal(1))
		Expect(pkgMatch.Files[0].Path).To(Equal("internal/indexer/query.go"))
	})

	It("normalizes and rejects blank queries", func() {
		file := model.FileSummary{
			Path:     "a/main.go",
			Language: "go",
			Symbols:  []model.Symbol{{Name: "Run", Kind: "function"}},
		}
		Expect(FileMatchesQuery(file, "run")).To(BeTrue())
		Expect(FileMatchesQuery(file, "missing")).To(BeFalse())
		Expect(NormalizeQuery(" HeLLo ")).To(Equal("hello"))
		Expect(MatchesQuery("Hello", "ell")).To(BeTrue())
		Expect(NormalizePath("./a/../b/main.go")).To(Equal("b/main.go"))

		result := testResult()
		Expect(SearchFiles(result, "   ")).To(BeNil())
		Expect(SearchSymbols(result, "")).To(BeNil())
		Expect(SearchWarnings(result, "")).To(HaveLen(len(result.Warnings)))
		_, ok := FindFile(result, " ")
		Expect(ok).To(BeFalse())
		_, ok = FindPackage(result, " ")
		Expect(ok).To(BeFalse())
		Expect(SearchSymbols(result, "go")).NotTo(BeEmpty())
	})
})
