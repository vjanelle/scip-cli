package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Integration fixtures", func() {
	fixtureRoot := filepath.Join("testdata", "fixtures", "polyglot")

	It("collects all supported fixture files", func() {
		req, err := NormalizeRequest(IndexRequest{
			Root:         fixtureRoot,
			SampleLimit:  16,
			MaxFileBytes: 1024,
			IncludeDeps:  true,
		})
		Expect(err).NotTo(HaveOccurred())

		files, warnings, err := CollectFiles(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(warnings).To(BeEmpty())
		Expect(files).To(ConsistOf(
			filepath.Join("go", "main.go"),
			filepath.Join("javascript", "app.js"),
			filepath.Join("typescript", "app.ts"),
			filepath.Join("python", "app.py"),
			filepath.Join("rust", "lib.rs"),
			filepath.Join("java", "App.java"),
			filepath.Join("zig", "main.zig"),
		))
	})

	It("has a fixture directory for every supported language", func() {
		entries, err := os.ReadDir(fixtureRoot)
		Expect(err).NotTo(HaveOccurred())

		dirs := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				dirs = append(dirs, entry.Name())
			}
		}

		Expect(dirs).To(ConsistOf(SupportedLanguages()))
	})

	DescribeTable("indexes non-Go fixture languages through dispatch",
		func(language, relativePath, symbolName string) {
			result, err := Dispatch(context.Background(), IndexRequest{
				Root:              fixtureRoot,
				Language:          language,
				SampleLimit:       8,
				MaxFiles:          4,
				MaxSymbolsPerFile: 4,
				SummaryDetail:     "normal",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.FilesIndexed).To(Equal(1))
			Expect(result.FileSummaries).To(HaveLen(1))
			Expect(filepath.ToSlash(result.FileSummaries[0].Path)).To(Equal(filepath.ToSlash(relativePath)))
			Expect(result.FileSummaries[0].Symbols).NotTo(BeEmpty())
			Expect(result.FileSummaries[0].Symbols[0].Name).To(Equal(symbolName))

			req, normalizeErr := NormalizeRequest(IndexRequest{
				Root:              fixtureRoot,
				Language:          language,
				SampleLimit:       8,
				MaxFiles:          4,
				MaxSymbolsPerFile: 4,
				SummaryDetail:     "normal",
			})
			Expect(normalizeErr).NotTo(HaveOccurred())

			prepared := PrepareResult(req, result)
			Expect(prepared.CompactFiles).To(HaveLen(1))
			Expect(prepared.CompactSymbols).NotTo(BeEmpty())
			Expect(prepared.StringTables.Paths).NotTo(BeEmpty())
		},
		Entry("javascript", "javascript", filepath.Join("javascript", "app.js"), "greet"),
		Entry("typescript", "typescript", filepath.Join("typescript", "app.ts"), "Greeter"),
		Entry("python", "python", filepath.Join("python", "app.py"), "greet"),
		Entry("rust", "rust", filepath.Join("rust", "lib.rs"), "greet"),
		Entry("java", "java", filepath.Join("java", "App.java"), "App"),
		Entry("zig", "zig", filepath.Join("zig", "main.zig"), "greet"),
	)

	It("indexes the Go fixture with stubbed scip-go and dependency loading", func() {
		req, err := NormalizeRequest(IndexRequest{
			Root:              fixtureRoot,
			Language:          "go",
			SampleLimit:       8,
			MaxFiles:          4,
			MaxSymbolsPerFile: 4,
			SummaryDetail:     "normal",
			IncludeDeps:       true,
		})
		Expect(err).NotTo(HaveOccurred())

		indexer := &GoIndexer{
			goBinary:   "go",
			scipBinary: "scip-go",
			runCommand: func(_ context.Context, dir, name string, args ...string) ([]byte, error) {
				Expect(dir).To(Equal(req.Root))
				switch name {
				case "scip-go":
					Expect(args).To(ContainElement("./..."))
					return []byte("indexed"), nil
				case "go":
					return fmt.Appendf(nil, "{\"Path\":\"example.com/polyglot\",\"Main\":true,\"Dir\":\"%s\"}\n{\"Path\":\"example.com/dep\",\"Version\":\"v1.0.0\",\"Indirect\":true}\n", filepath.ToSlash(req.Root)), nil
				default:
					return nil, fmt.Errorf("unexpected command: %s", name)
				}
			},
		}

		result, err := indexer.Index(context.Background(), IndexRequest{
			Root:              fixtureRoot,
			Language:          "go",
			SampleLimit:       8,
			MaxFiles:          4,
			MaxSymbolsPerFile: 4,
			SummaryDetail:     "normal",
			IncludeDeps:       true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Indexer).To(Equal("scip-go"))
		Expect(result.FileSummaries).To(HaveLen(1))
		Expect(filepath.ToSlash(result.FileSummaries[0].Path)).To(Equal("go/main.go"))
		Expect(result.Dependencies).To(HaveLen(2))
		Expect(result.Warnings).To(ContainElement("indexed"))

		prepared := PrepareResult(req, result)
		Expect(prepared.CompactFiles).To(HaveLen(1))
		Expect(prepared.Packages).NotTo(BeEmpty())
		Expect(prepared.ResultHandle).To(BeEmpty())
	})

})
