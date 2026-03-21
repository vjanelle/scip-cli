//go:build cgo

package indexer

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tree-sitter helper coverage", func() {
	It("covers fallback symbols and grammar selection", func() {
		Expect(NewTreeSitterIndexer()).NotTo(BeNil())
		Expect((treeSitterIndexCommand{}).Name()).To(Equal("tree-sitter"))
		Expect((treeSitterIndexCommand{}).Supports("python", IndexRequest{})).To(BeTrue())
		Expect((treeSitterIndexCommand{}).Supports("go", IndexRequest{})).To(BeFalse())

		symbols := fallbackSymbols("app.py", []byte("def greet():\n    pass\nclass Person:\n    pass\n"))
		Expect(symbols).NotTo(BeEmpty())
		Expect(symbols[0].StartLine).To(BeNumerically(">=", 1))

		language, err := grammarForFile("app.tsx")
		Expect(err).NotTo(HaveOccurred())
		Expect(language).NotTo(BeNil())

		_, err = grammarForFile("README.txt")
		Expect(err).To(MatchError(ContainSubstring("no tree-sitter grammar configured")))
	})

	It("covers summarizeFile fallback and file error branches", func() {
		tempRoot := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(tempRoot, "app.txt"), []byte("func greet() {}\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tempRoot, "app.ts"), []byte("function greet(): string {\n  return 'hi';\n}\n"), 0o644)).To(Succeed())

		summary, doc, err := NewTreeSitterIndexer().summarizeFile(context.Background(), tempRoot, "app.txt", true)
		Expect(err).NotTo(HaveOccurred())
		Expect(summary.Symbols).NotTo(BeEmpty())
		Expect(doc.Text).To(BeEmpty())

		_, _, err = NewTreeSitterIndexer().summarizeFile(context.Background(), tempRoot, "missing.ts", false)
		Expect(err).To(HaveOccurred())

		result, err := (treeSitterIndexCommand{}).Execute(context.Background(), IndexRequest{Root: tempRoot, Language: "typescript", Indexer: "tree-sitter"})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Indexer).To(Equal("tree-sitter"))
	})
})
