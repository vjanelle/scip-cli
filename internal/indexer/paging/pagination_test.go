package paging

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/vjanelle/scip-cli/internal/indexer/model"
)

var _ = Describe("PaginateResult", func() {
	It("returns the first page and a token-based follow-up", func() {
		result, err := PaginateResult(model.Result{
			FileSummaries: []model.FileSummary{{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}},
			Dependencies:  []model.ModuleDependency{{Path: "dep-a"}, {Path: "dep-b"}},
		}, model.IndexRequest{PageSize: 2})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.ReturnedFiles).To(Equal(2))
		Expect(result.ReturnedDeps).To(Equal(2))
		Expect(result.TotalFiles).To(Equal(3))
		Expect(result.TotalDeps).To(Equal(2))
		Expect(result.NextPageToken).NotTo(BeEmpty())

		token, err := EncodePageToken(Cursor{FileOffset: 1, DepOffset: 1})
		Expect(err).NotTo(HaveOccurred())
		result, err = PaginateResult(model.Result{
			FileSummaries: []model.FileSummary{{Path: "a"}, {Path: "b"}, {Path: "c"}},
			Dependencies:  []model.ModuleDependency{{Path: "d1"}, {Path: "d2"}},
		}, model.IndexRequest{PageSize: 1, PageToken: token})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.FileSummaries).To(HaveLen(1))
		Expect(result.FileSummaries[0].Path).To(Equal("b"))
		Expect(result.Dependencies).To(HaveLen(1))
		Expect(result.Dependencies[0].Path).To(Equal("d2"))
	})

	It("covers token helpers and compact paging", func() {
		token, err := EncodePageToken(Cursor{FileOffset: 2, DepOffset: 3})
		Expect(err).NotTo(HaveOccurred())
		cursor, err := DecodePageToken(token)
		Expect(err).NotTo(HaveOccurred())
		Expect(cursor).To(Equal(Cursor{FileOffset: 2, DepOffset: 3}))

		_, err = PaginateResult(model.Result{}, model.IndexRequest{
			PageSize:  1,
			PageToken: base64.RawURLEncoding.EncodeToString([]byte("{bad")),
		})
		Expect(err).To(HaveOccurred())

		result, err := PaginateResult(model.Result{
			FileSummaries: []model.FileSummary{{Path: "a"}, {Path: "b"}, {Path: "c"}},
			CompactFiles: []model.CompactFileSummary{
				{HandleID: "file:a", SymbolRefs: []int{1}},
				{HandleID: "file:b", SymbolRefs: []int{2}},
				{HandleID: "file:c", SymbolRefs: []int{3}},
			},
			CompactSymbols: []model.CompactSymbol{{ID: 1}, {ID: 2}, {ID: 3}},
			Relationships: []model.Relationship{
				{From: "file:a", To: "symbol:1", Kind: "contains"},
				{From: "file:b", To: "symbol:2", Kind: "contains"},
				{From: "file:c", To: "symbol:3", Kind: "contains"},
			},
		}, model.IndexRequest{PageSize: 1})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.CompactFiles).To(HaveLen(1))
		Expect(result.CompactFiles[0].HandleID).To(Equal("file:a"))
		Expect(result.CompactSymbols).To(HaveLen(1))
		Expect(result.CompactSymbols[0].ID).To(Equal(1))
		Expect(result.Relationships).To(HaveLen(1))
	})
})
