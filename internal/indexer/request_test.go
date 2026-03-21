package indexer

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NormalizeRequest", func() {
	It("normalizes the root and defaults", func() {
		tempRoot := GinkgoT().TempDir()

		req, err := NormalizeRequest(IndexRequest{
			Root:     tempRoot,
			Language: "golang",
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(req.Root).To(Equal(tempRoot))
		Expect(req.Language).To(Equal("go"))
		Expect(req.SampleLimit).To(BeNumerically(">", 0))
		Expect(req.PageSize).To(BeNumerically(">", 0))
		Expect(req.IncludeDeps).To(BeTrue())
		Expect(req.MaxFileBytes).To(BeNumerically(">", 0))
		Expect(req.ResponseMode).To(Equal("compact"))
		Expect(req.AutoResponseMode).NotTo(BeNil())
		Expect(*req.AutoResponseMode).To(BeTrue())
		Expect(req.AutoHandleFileThreshold).To(Equal(DefaultAutoHandleFileThreshold))
		Expect(req.AutoHandleSymbolThreshold).To(Equal(DefaultAutoHandleSymbolThreshold))
	})

	It("rejects an empty root", func() {
		_, err := NormalizeRequest(IndexRequest{})
		Expect(err).To(HaveOccurred())
	})

	It("normalizes common language aliases", func() {
		Expect(normalizeLanguage("ts")).To(Equal("typescript"))
		Expect(normalizeLanguage("node")).To(Equal("javascript"))
		Expect(normalizeLanguage("py")).To(Equal("python"))
		Expect(normalizeLanguage("golang")).To(Equal("go"))
	})

	It("normalizes backend aliases and repeated args", func() {
		Expect(normalizeIndexer("auto")).To(BeEmpty())
		Expect(normalizeIndexer("scip-go")).To(Equal("go"))
		Expect(normalizeIndexer("symbolic-fallback")).To(Equal("tree-sitter"))
		Expect(normalizeIndexer("lsp")).To(Equal("scip-lsp"))
		Expect(cleanedArgs([]string{" one ", "", "two"})).To(Equal([]string{"one", "two"}))
	})

	It("normalizes response mode values", func() {
		tempRoot := GinkgoT().TempDir()

		req, err := NormalizeRequest(IndexRequest{
			Root:          tempRoot,
			ResponseMode:  "HANDLES",
			SummaryDetail: "deep",
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(req.ResponseMode).To(Equal("handles"))
		Expect(req.SummaryDetail).To(Equal("normal"))
	})

	It("preserves explicit adaptive mode overrides", func() {
		tempRoot := GinkgoT().TempDir()
		disabled := false

		req, err := NormalizeRequest(IndexRequest{
			Root:                      tempRoot,
			AutoResponseMode:          &disabled,
			AutoHandleFileThreshold:   220,
			AutoHandleSymbolThreshold: 4200,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(req.AutoResponseMode).NotTo(BeNil())
		Expect(*req.AutoResponseMode).To(BeFalse())
		Expect(req.AutoHandleFileThreshold).To(Equal(220))
		Expect(req.AutoHandleSymbolThreshold).To(Equal(4200))
	})
})

var _ = Describe("CollectFiles", func() {
	It("collects supported files and ignores hidden directories", func() {
		tempRoot := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(tempRoot, "main.go"), []byte("package main"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tempRoot, "notes.txt"), []byte("ignore"), 0o644)).To(Succeed())
		Expect(os.Mkdir(filepath.Join(tempRoot, ".git"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tempRoot, ".git", "config"), []byte("ignored"), 0o644)).To(Succeed())

		files, warnings, err := CollectFiles(IndexRequest{
			Root:         tempRoot,
			SampleLimit:  10,
			MaxFileBytes: 1024,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(warnings).To(BeEmpty())
		Expect(files).To(Equal([]string{"main.go"}))
	})

	It("includes vendor and node_modules sources", func() {
		tempRoot := GinkgoT().TempDir()
		Expect(os.MkdirAll(filepath.Join(tempRoot, "vendor", "acme"), 0o755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(tempRoot, "node_modules", "pkg"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tempRoot, "vendor", "acme", "lib.go"), []byte("package acme"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tempRoot, "node_modules", "pkg", "index.js"), []byte("export const x = 1"), 0o644)).To(Succeed())

		files, warnings, err := CollectFiles(IndexRequest{
			Root:         tempRoot,
			SampleLimit:  10,
			MaxFileBytes: 1024,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(warnings).To(BeEmpty())
		Expect(files).To(ContainElements(
			filepath.Join("vendor", "acme", "lib.go"),
			filepath.Join("node_modules", "pkg", "index.js"),
		))
	})

	It("respects language filters and missing path warnings", func() {
		tempRoot := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(tempRoot, "main.go"), []byte("package main"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tempRoot, "app.py"), []byte("def main(): pass"), 0o644)).To(Succeed())

		files, warnings, err := CollectFiles(IndexRequest{
			Root:         tempRoot,
			Language:     "python",
			Paths:        []string{"missing", "."},
			SampleLimit:  10,
			MaxFileBytes: 1024,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(Equal([]string{"app.py"}))
		Expect(warnings).NotTo(BeEmpty())
	})

	It("exposes the directory and file filter wrappers", func() {
		tempRoot := GinkgoT().TempDir()
		hiddenDirReq := IndexRequest{IncludeHidden: false}
		Expect(shouldSkipDir(hiddenDirReq, ".git")).To(BeTrue())
		Expect(shouldSkipDir(hiddenDirReq, "pkg")).To(BeFalse())

		hiddenPath := filepath.Join(tempRoot, ".hidden.py")
		Expect(os.WriteFile(hiddenPath, []byte("print('hidden')"), 0o644)).To(Succeed())
		hiddenInfo, err := os.Stat(hiddenPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(includePath(IndexRequest{MaxFileBytes: 1024}, ".hidden.py", hiddenInfo)).To(BeFalse())

		goPath := filepath.Join(tempRoot, "main.go")
		Expect(os.WriteFile(goPath, []byte("package main"), 0o644)).To(Succeed())
		goInfo, err := os.Stat(goPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(includePath(IndexRequest{Language: "go", MaxFileBytes: 1024}, "main.go", goInfo)).To(BeTrue())
		Expect(includePath(IndexRequest{Language: "python", MaxFileBytes: 1024}, "main.go", goInfo)).To(BeFalse())
	})
})

var _ = Describe("Dispatch", func() {
	It("indexes non-go sources with the symbolic fallback on this platform", func() {
		tempRoot := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(tempRoot, "app.py"), []byte("def greet():\n    return 'hi'\n"), 0o644)).To(Succeed())

		result, err := Dispatch(GinkgoT().Context(), IndexRequest{
			Root:        tempRoot,
			Language:    "python",
			SampleLimit: 10,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.FilesIndexed).To(Equal(1))
		Expect(result.FileSummaries).To(HaveLen(1))
		Expect(result.FileSummaries[0].Symbols).NotTo(BeEmpty())
	})

	It("emits a security warning for invisible Unicode characters", func() {
		tempRoot := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(tempRoot, "app.py"), []byte("def\u200bgreet():\n    return 'hi'\n"), 0o644)).To(Succeed())

		result, err := Dispatch(GinkgoT().Context(), IndexRequest{
			Root:        tempRoot,
			Language:    "python",
			SampleLimit: 10,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Warnings).To(ContainElement(ContainSubstring("security warning")))
		Expect(result.Warnings).To(ContainElement(ContainSubstring("invisible Unicode character")))
	})

	It("detects language from a single path", func() {
		Expect(pickLanguage(IndexRequest{Paths: []string{"pkg/main.go"}})).To(Equal("go"))
		Expect(pickLanguage(IndexRequest{Language: "python"})).To(Equal("python"))
	})

	It("detects file extensions", func() {
		Expect(DetectLanguage("main.go")).To(Equal("go"))
		Expect(DetectLanguage("index.tsx")).To(Equal("typescript"))
		Expect(DetectLanguage("script.js")).To(Equal("javascript"))
		Expect(DetectLanguage("unknown.txt")).To(Equal(""))
	})

	It("auto-selects scip-lsp when lspCommand is configured", func() {
		tempRoot := GinkgoT().TempDir()
		req, err := NormalizeRequest(IndexRequest{
			Root:       tempRoot,
			Language:   "python",
			LSPCommand: os.Args[0],
		})
		Expect(err).NotTo(HaveOccurred())

		command, err := commandForRequest(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(command.Name()).To(Equal("scip-lsp"))
	})

	It("keeps go as the default backend even when lspCommand is configured", func() {
		tempRoot := GinkgoT().TempDir()
		req, err := NormalizeRequest(IndexRequest{
			Root:       tempRoot,
			Language:   "go",
			LSPCommand: os.Args[0],
		})
		Expect(err).NotTo(HaveOccurred())

		command, err := commandForRequest(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(command.Name()).To(Equal("go"))
	})
})
