package indexer

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PaginateResult", func() {
	It("returns the first page and a next token", func() {
		result, err := PaginateResult(Result{
			FileSummaries: []FileSummary{
				{Path: "a.go"},
				{Path: "b.go"},
				{Path: "c.go"},
			},
			Dependencies: []ModuleDependency{
				{Path: "dep-a"},
				{Path: "dep-b"},
			},
		}, IndexRequest{PageSize: 2})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.ReturnedFiles).To(Equal(2))
		Expect(result.ReturnedDeps).To(Equal(2))
		Expect(result.TotalFiles).To(Equal(3))
		Expect(result.TotalDeps).To(Equal(2))
		Expect(result.NextPageToken).NotTo(BeEmpty())
	})

	It("uses the supplied page token", func() {
		token, err := encodePageToken(pageCursor{FileOffset: 1, DepOffset: 1})
		Expect(err).NotTo(HaveOccurred())

		result, err := PaginateResult(Result{
			FileSummaries: []FileSummary{{Path: "a"}, {Path: "b"}, {Path: "c"}},
			Dependencies:  []ModuleDependency{{Path: "d1"}, {Path: "d2"}},
		}, IndexRequest{PageSize: 1, PageToken: token})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.FileSummaries).To(HaveLen(1))
		Expect(result.FileSummaries[0].Path).To(Equal("b"))
		Expect(result.Dependencies).To(HaveLen(1))
		Expect(result.Dependencies[0].Path).To(Equal("d2"))
	})

	It("rejects malformed page tokens", func() {
		_, err := PaginateResult(Result{}, IndexRequest{
			PageSize:  1,
			PageToken: base64.RawURLEncoding.EncodeToString([]byte("{bad")),
		})
		Expect(err).To(HaveOccurred())
	})

	It("pages compact files and their referenced symbols together", func() {
		result, err := PaginateResult(Result{
			FileSummaries: []FileSummary{{Path: "a"}, {Path: "b"}, {Path: "c"}},
			CompactFiles: []CompactFileSummary{
				{HandleID: "file:a", SymbolRefs: []int{1}},
				{HandleID: "file:b", SymbolRefs: []int{2}},
				{HandleID: "file:c", SymbolRefs: []int{3}},
			},
			CompactSymbols: []CompactSymbol{
				{ID: 1},
				{ID: 2},
				{ID: 3},
			},
			Relationships: []Relationship{
				{From: "file:a", To: "symbol:1", Kind: "contains"},
				{From: "file:b", To: "symbol:2", Kind: "contains"},
				{From: "file:c", To: "symbol:3", Kind: "contains"},
			},
		}, IndexRequest{PageSize: 1})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.CompactFiles).To(HaveLen(1))
		Expect(result.CompactFiles[0].HandleID).To(Equal("file:a"))
		Expect(result.CompactSymbols).To(HaveLen(1))
		Expect(result.CompactSymbols[0].ID).To(Equal(1))
		Expect(result.Relationships).To(HaveLen(1))
	})
})
