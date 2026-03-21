package workspace

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/vjanelle/scip-cli/internal/indexer/model"
)

var _ = Describe("NormalizeRequest", func() {
	It("normalizes defaults and aliases", func() {
		tempRoot := GinkgoT().TempDir()
		req, err := NormalizeRequest(model.IndexRequest{
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

		_, err = NormalizeRequest(model.IndexRequest{})
		Expect(err).To(HaveOccurred())
		Expect(NormalizeLanguage("ts")).To(Equal("typescript"))
		Expect(NormalizeLanguage("node")).To(Equal("javascript"))
		Expect(NormalizeLanguage("py")).To(Equal("python"))
		Expect(NormalizeLanguage("golang")).To(Equal("go"))
		Expect(NormalizeIndexer("auto")).To(BeEmpty())
		Expect(NormalizeIndexer("scip-go")).To(Equal("go"))
		Expect(NormalizeIndexer("symbolic-fallback")).To(Equal("tree-sitter"))
		Expect(NormalizeIndexer("lsp")).To(Equal("scip-lsp"))
		Expect(CleanedArgs([]string{" one ", "", "two"})).To(Equal([]string{"one", "two"}))
	})

	It("normalizes response modes and override values", func() {
		tempRoot := GinkgoT().TempDir()
		req, err := NormalizeRequest(model.IndexRequest{
			Root:          tempRoot,
			ResponseMode:  "HANDLES",
			SummaryDetail: "deep",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(req.ResponseMode).To(Equal("handles"))
		Expect(req.SummaryDetail).To(Equal("normal"))

		disabled := false
		req, err = NormalizeRequest(model.IndexRequest{
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
	It("collects visible supported files", func() {
		tempRoot := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(tempRoot, "main.go"), []byte("package main"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tempRoot, "notes.txt"), []byte("ignore"), 0o644)).To(Succeed())
		Expect(os.Mkdir(filepath.Join(tempRoot, ".git"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tempRoot, ".git", "config"), []byte("ignored"), 0o644)).To(Succeed())

		files, warnings, err := CollectFiles(model.IndexRequest{Root: tempRoot, SampleLimit: 10, MaxFileBytes: 1024})
		Expect(err).NotTo(HaveOccurred())
		Expect(warnings).To(BeEmpty())
		Expect(files).To(Equal([]string{"main.go"}))
	})

	It("covers filters, helpers, and supported languages", func() {
		tempRoot := GinkgoT().TempDir()
		Expect(os.MkdirAll(filepath.Join(tempRoot, "vendor", "acme"), 0o755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(tempRoot, "node_modules", "pkg"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tempRoot, "vendor", "acme", "lib.go"), []byte("package acme"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tempRoot, "node_modules", "pkg", "index.js"), []byte("export const x = 1"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tempRoot, "main.go"), []byte("package main"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tempRoot, "app.py"), []byte("def main(): pass"), 0o644)).To(Succeed())

		files, warnings, err := CollectFiles(model.IndexRequest{Root: tempRoot, SampleLimit: 10, MaxFileBytes: 1024})
		Expect(err).NotTo(HaveOccurred())
		Expect(warnings).To(BeEmpty())
		Expect(len(files)).To(BeNumerically(">=", 3))

		files, warnings, err = CollectFiles(model.IndexRequest{
			Root:         tempRoot,
			Language:     "python",
			Paths:        []string{"missing", "."},
			SampleLimit:  10,
			MaxFileBytes: 1024,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(Equal([]string{"app.py"}))
		Expect(warnings).NotTo(BeEmpty())

		hiddenPath := filepath.Join(tempRoot, ".hidden.py")
		Expect(os.WriteFile(hiddenPath, []byte("print('hidden')"), 0o644)).To(Succeed())
		hiddenInfo, err := os.Stat(hiddenPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(ShouldSkipDir(model.IndexRequest{IncludeHidden: false}, ".git")).To(BeTrue())
		Expect(ShouldSkipDir(model.IndexRequest{IncludeHidden: false}, "pkg")).To(BeFalse())
		Expect(IncludePath(model.IndexRequest{MaxFileBytes: 1024}, ".hidden.py", hiddenInfo)).To(BeFalse())

		goInfo, err := os.Stat(filepath.Join(tempRoot, "main.go"))
		Expect(err).NotTo(HaveOccurred())
		Expect(IncludePath(model.IndexRequest{Language: "go", MaxFileBytes: 1024}, "main.go", goInfo)).To(BeTrue())
		Expect(IncludePath(model.IndexRequest{Language: "python", MaxFileBytes: 1024}, "main.go", goInfo)).To(BeFalse())

		Expect(SupportedLanguages()).To(ContainElement("go"))
		Expect(PackageNameForPath("main.go")).To(Equal("root"))
		Expect(PackageNameForPath("pkg/main.go")).To(Equal("pkg"))
	})
})
