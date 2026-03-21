package main

import (
	"bytes"
	"errors"
	"path/filepath"

	"github.com/gomarkdown/markdown/ast"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/vjanelle/scip-cli/internal/indexer"
	bundledskills "github.com/vjanelle/scip-cli/skills"
)

type failingWriter struct {
	fail error
}

func (writer failingWriter) Write([]byte) (int, error) {
	return 0, writer.fail
}

var _ = Describe("CLI helper coverage", func() {
	It("recovers render writer failures and preserves unexpected panics", func() {
		writeErr := errors.New("boom")
		err := withRenderWriter(failingWriter{fail: writeErr}, func(writer renderWriter) {
			writer.Fprintf("hello %s", "world")
		})
		Expect(err).To(MatchError(writeErr))

		Expect(func() {
			_ = withRenderWriter(&bytes.Buffer{}, func(renderWriter) {
				panic("unexpected")
			})
		}).To(PanicWith("unexpected"))
	})

	It("covers render writer println failures", func() {
		writeErr := errors.New("println failed")
		err := withRenderWriter(failingWriter{fail: writeErr}, func(writer renderWriter) {
			writer.Fprintln("hello")
		})
		Expect(err).To(MatchError(writeErr))
	})

	It("renders search and skill text formats", func() {
		stdout := &bytes.Buffer{}
		Expect(writeTextSearchFiles(stdout, searchFilesOutput{
			Query:        "main",
			TotalMatches: 1,
			Files: []indexer.FileMatch{{
				Package: "cmd/scip-cli",
				File:    indexer.FileSummary{Path: "cmd/scip-cli/main.go"},
			}},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Query: main"))
		Expect(stdout.String()).To(ContainSubstring("cmd/scip-cli/main.go"))

		stdout.Reset()
		Expect(writeTextSearchSymbols(stdout, searchSymbolsOutput{
			Query:        "run",
			TotalMatches: 1,
			Symbols:      []indexer.Symbol{{Name: "runCLI", Kind: "function", Path: "cmd/scip-cli/main.go"}},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("- runCLI [function] cmd/scip-cli/main.go"))

		stdout.Reset()
		Expect(writeTextSearchWarnings(stdout, searchWarningsOutput{
			Query:        "unicode",
			TotalMatches: 1,
			Warnings:     []string{"warning: unicode"},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("warning: unicode"))

		stdout.Reset()
		Expect(writeTextSkillInstructions(stdout, skillInstallFixture())).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Skill: scip-cli"))
		Expect(stdout.String()).To(ContainSubstring("Files:"))
	})

	It("runs bundled skill instructions across formats and errors", func() {
		stdout := &bytes.Buffer{}
		Expect(runSkillInstallInstructions(skillInstallOptions{
			name:   "scip-cli",
			format: string(outputFormatJSON),
		}, stdout)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring(`"name":"scip-cli"`))

		stdout.Reset()
		Expect(runSkillInstallInstructions(skillInstallOptions{
			name:   "scip-cli",
			format: string(outputFormatMarkdown),
		}, stdout)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("# Skill Install Instructions"))

		err := runSkillInstallInstructions(skillInstallOptions{
			name:   "missing-skill",
			format: string(outputFormatJSON),
		}, &bytes.Buffer{})
		Expect(err).To(MatchError(ContainSubstring("unknown bundled skill")))

		err = runSkillInstallInstructions(skillInstallOptions{
			name:   "scip-cli",
			format: "bogus",
		}, &bytes.Buffer{})
		Expect(err).To(MatchError(ContainSubstring("unsupported output format")))
	})

	It("runs warning searches and search renderers through their success paths", func() {
		fixtureRoot := filepath.Join("..", "..", "internal", "indexer", "testdata", "fixtures", "polyglot")
		stdout := &bytes.Buffer{}
		Expect(runSearchWarnings(searchOptions{
			index: indexOptions{root: fixtureRoot, format: string(outputFormatText)},
			query: "unicode",
		}, stdout)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Query: unicode"))

		stdout.Reset()
		Expect(renderSearchFilesOutput(stdout, indexOptions{format: string(outputFormatJSON), pretty: false}, searchFilesOutput{
			Query:        "main",
			TotalMatches: 1,
			Files:        []indexer.FileMatch{{Package: "cmd", File: indexer.FileSummary{Path: "cmd/main.go"}}},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("\"totalMatches\":1"))

		stdout.Reset()
		Expect(renderSearchFilesOutput(stdout, indexOptions{format: string(outputFormatText)}, searchFilesOutput{
			Query:        "main",
			TotalMatches: 1,
			Files:        []indexer.FileMatch{{Package: "cmd", File: indexer.FileSummary{Path: "cmd/main.go"}}},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Matches: 1"))

		stdout.Reset()
		Expect(renderSearchFilesOutput(stdout, indexOptions{format: string(outputFormatMarkdown)}, searchFilesOutput{
			Query:        "main",
			TotalMatches: 1,
			Files:        []indexer.FileMatch{{Package: "cmd", File: indexer.FileSummary{Path: "cmd/main.go"}}},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("# File Search"))

		stdout.Reset()
		Expect(renderSearchSymbolsOutput(stdout, indexOptions{format: string(outputFormatMarkdown)}, searchSymbolsOutput{
			Query:        "run",
			TotalMatches: 1,
			Symbols:      []indexer.Symbol{{Name: "runCLI", Kind: "function", Path: "cmd/main.go", StartLine: 1, EndLine: 2}},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("# Symbol Search"))

		stdout.Reset()
		Expect(renderSearchSymbolsOutput(stdout, indexOptions{format: string(outputFormatText)}, searchSymbolsOutput{
			Query:        "run",
			TotalMatches: 1,
			Symbols:      []indexer.Symbol{{Name: "runCLI", Kind: "function", Path: "cmd/main.go", StartLine: 1, EndLine: 2}},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("- runCLI [function] cmd/main.go"))

		stdout.Reset()
		Expect(renderSearchSymbolsOutput(stdout, indexOptions{format: string(outputFormatJSON), pretty: false}, searchSymbolsOutput{
			Query:        "run",
			TotalMatches: 1,
			Symbols:      []indexer.Symbol{{Name: "runCLI", Kind: "function", Path: "cmd/main.go", StartLine: 1, EndLine: 2}},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("\"query\":\"run\""))

		stdout.Reset()
		Expect(renderSearchWarningsOutput(stdout, indexOptions{format: string(outputFormatText)}, searchWarningsOutput{
			Query:        "unicode",
			TotalMatches: 1,
			Warnings:     []string{"warning: unicode"},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("warning: unicode"))

		stdout.Reset()
		Expect(renderSearchWarningsOutput(stdout, indexOptions{format: string(outputFormatMarkdown)}, searchWarningsOutput{
			Query:        "unicode",
			TotalMatches: 1,
			Warnings:     []string{"warning: unicode"},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("# Warning Search"))

		stdout.Reset()
		Expect(renderSearchWarningsOutput(stdout, indexOptions{format: string(outputFormatJSON), pretty: false}, searchWarningsOutput{
			Query:        "unicode",
			TotalMatches: 1,
			Warnings:     []string{"warning: unicode"},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("\"warnings\":[\"warning: unicode\"]"))
	})

	It("renders search and package markdown formats", func() {
		stdout := &bytes.Buffer{}
		Expect(writeMarkdownSearchSymbols(stdout, searchSymbolsOutput{
			Query:        "run",
			TotalMatches: 1,
			Symbols:      []indexer.Symbol{{Name: "runCLI", Kind: "function", Path: "cmd/scip-cli/main.go", StartLine: 10, EndLine: 20}},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("# Symbol Search"))
		Expect(stdout.String()).To(ContainSubstring("lines: `10-20`"))

		stdout.Reset()
		Expect(writeMarkdownSearchWarnings(stdout, searchWarningsOutput{
			Query:        "unicode",
			TotalMatches: 1,
			Warnings:     []string{"warning: unicode"},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("# Warning Search"))

		stdout.Reset()
		Expect(writeMarkdownShowPackage(stdout, showPackageOutput{
			Name: "cmd/scip-cli",
			Match: indexer.PackageMatch{
				Package: indexer.PackageSummary{Name: "cmd/scip-cli", FileCount: 1, SymbolCount: 2},
				Files: []indexer.FileSummary{{
					Path:     "cmd/scip-cli/main.go",
					Language: "go",
					Bytes:    12,
					Symbols:  []indexer.Symbol{{Name: "runCLI"}},
				}},
			},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("# Package Detail"))
		Expect(stdout.String()).To(ContainSubstring("cmd/scip-cli/main.go"))
	})

	It("renders show outputs across formats and package lookup paths", func() {
		fileOutput := showFileOutput{
			Path: "pkg/app.go",
			Match: indexer.FileMatch{
				Package: "pkg",
				File: indexer.FileSummary{
					Path:     "pkg/app.go",
					Language: "go",
					Bytes:    42,
					Symbols: []indexer.Symbol{{
						Name:      "Run",
						Kind:      "function",
						Path:      "pkg/app.go",
						StartLine: 1,
						EndLine:   4,
					}},
				},
			},
		}
		stdout := &bytes.Buffer{}
		Expect(renderShowFileOutput(stdout, indexOptions{format: string(outputFormatText)}, fileOutput)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("- Run [function] 1-4"))

		stdout.Reset()
		Expect(renderShowFileOutput(stdout, indexOptions{format: string(outputFormatMarkdown)}, fileOutput)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("# File Detail"))

		stdout.Reset()
		Expect(renderShowFileOutput(stdout, indexOptions{format: string(outputFormatJSON), pretty: false}, fileOutput)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring(`"path":"pkg/app.go"`))

		packageOutput := showPackageOutput{
			Name: "pkg",
			Match: indexer.PackageMatch{
				Package: indexer.PackageSummary{Name: "pkg", FileCount: 1, SymbolCount: 1},
				Files:   []indexer.FileSummary{{Path: "pkg/app.go", Language: "go", Bytes: 42}},
			},
		}
		stdout.Reset()
		Expect(renderShowPackageOutput(stdout, indexOptions{format: string(outputFormatText)}, packageOutput)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Package: pkg"))

		stdout.Reset()
		Expect(renderShowPackageOutput(stdout, indexOptions{format: string(outputFormatMarkdown)}, packageOutput)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("# Package Detail"))

		stdout.Reset()
		Expect(renderShowPackageOutput(stdout, indexOptions{format: string(outputFormatJSON), pretty: false}, packageOutput)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring(`"name":"pkg"`))
	})

	It("renders index output helpers and compact file sections", func() {
		result := indexer.Result{
			Indexer:      "tree-sitter",
			Root:         "/workspace",
			Language:     "go",
			FilesScanned: 3,
			FilesIndexed: 2,
			FilesSkipped: 1,
			Warnings:     []string{"warning"},
			Dependencies: []indexer.ModuleDependency{{Path: "example.com/dep"}},
			CompactFiles: []indexer.CompactFileSummary{{
				PathID:     1,
				LanguageID: 2,
				Bytes:      99,
				SymbolRefs: []int{7},
			}},
			CompactSymbols: []indexer.CompactSymbol{{ID: 7, NameID: 3}},
			StringTables: indexer.StringTables{
				Paths:       []indexer.StringTableEntry{{ID: 1, Value: "pkg/app.go"}},
				Languages:   []indexer.StringTableEntry{{ID: 2, Value: "go"}},
				SymbolNames: []indexer.StringTableEntry{{ID: 3, Value: "runCLI"}},
			},
		}

		stdout := &bytes.Buffer{}
		Expect(writeTextIndexResult(stdout, result)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Dependencies: 1"))
		Expect(stdout.String()).To(ContainSubstring("Warnings:"))
		Expect(stdout.String()).To(ContainSubstring("symbols: runCLI"))

		section := buildIndexFilesSection(result)
		Expect(section.Items).To(HaveLen(1))
		Expect(section.Items[0].Children[0]).To(ContainSubstring("runCLI"))
		Expect(buildStringSection("Warnings", []string{"a"}, func(value string) string { return value }).Heading).To(Equal("Warnings"))
		Expect(joinMarkdownCodes([]string{"one", "two"})).To(Equal("`one`, `two`"))
		Expect(symbolNames([]indexer.Symbol{{Name: "beta"}, {Name: ""}, {Name: "alpha"}})).To(Equal([]string{"alpha", "beta"}))
		Expect(tableEntriesByID([]indexer.StringTableEntry{{ID: 2, Value: "go"}})).To(HaveKeyWithValue(2, "go"))
		Expect(compactSymbolsByID([]indexer.CompactSymbol{{ID: 9, NameID: 2}})).To(HaveKey(9))
	})

	It("renders index results for detailed file summaries and json output", func() {
		result := indexer.Result{
			Indexer:       "tree-sitter",
			Root:          "/workspace",
			FilesScanned:  1,
			FilesIndexed:  1,
			FileSummaries: []indexer.FileSummary{{Path: "pkg/app.go", Language: "go", Bytes: 55, Symbols: []indexer.Symbol{{Name: "Run"}}}},
			SCIPPath:      "/workspace/index.scip",
			NextPageToken: "next",
		}
		stdout := &bytes.Buffer{}
		Expect(renderIndexResult(stdout, indexOptions{format: string(outputFormatText)}, result)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Top files:"))
		Expect(stdout.String()).To(ContainSubstring("symbols: Run"))

		stdout.Reset()
		Expect(renderIndexResult(stdout, indexOptions{format: string(outputFormatMarkdown)}, result)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("# Index Result"))
		Expect(stdout.String()).To(ContainSubstring("SCIP path"))

		stdout.Reset()
		Expect(renderIndexResult(stdout, indexOptions{format: string(outputFormatJSON), pretty: false}, result)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring(`"nextPageToken":"next"`))
	})

	It("covers renderer format validation branches", func() {
		format, err := normalizeOutputFormat("md")
		Expect(err).NotTo(HaveOccurred())
		Expect(format).To(Equal(outputFormatMarkdown))
		Expect(codeFenceLanguage("note.md")).To(Equal("markdown"))
		Expect(codeFenceLanguage("config.yaml")).To(Equal("yaml"))
		Expect(codeFenceLanguage("plain.txt")).To(Equal("text"))
		Expect(markdownCode("value")).To(Equal("`value`"))
		Expect(markdownCode("const message = `hello`")).To(Equal("`` const message = `hello` ``"))
		Expect(markdownCode("``tick``")).To(Equal("``` ``tick`` ```"))
		Expect(markdownText("const message = `hello`; [ok]")).To(Equal("const message = \\`hello\\`; \\[ok\\]"))

		err = renderSearchWarningsOutput(&bytes.Buffer{}, indexOptions{format: "bogus"}, searchWarningsOutput{})
		Expect(err).To(MatchError(ContainSubstring("unsupported output format")))

		err = renderShowPackageOutput(&bytes.Buffer{}, indexOptions{format: "bogus"}, showPackageOutput{})
		Expect(err).To(MatchError(ContainSubstring("unsupported output format")))

		err = renderIndexResult(&bytes.Buffer{}, indexOptions{format: "bogus"}, indexer.Result{})
		Expect(err).To(MatchError(ContainSubstring("unsupported output format")))
	})

	It("covers show helpers and package command edge cases", func() {
		rangeValue, err := parseShowFileRange("")
		Expect(err).NotTo(HaveOccurred())
		Expect(rangeValue).To(BeNil())

		rangeValue, err = parseShowFileRange("3-7")
		Expect(err).NotTo(HaveOccurred())
		Expect(rangeValue.start).To(Equal(3))
		Expect(rangeValue.end).To(Equal(7))

		Expect(splitSourceLines("a\nb\n")).To(Equal([]string{"a", "b"}))
		Expect(splitSourceLines("a\nb")).To(Equal([]string{"a", "b"}))
		Expect(splitSourceLines("a\r\nb\r\n")).To(Equal([]string{"a", "b"}))

		_, err = buildShowFileRangeView(showFileOptions{
			index: indexOptions{root: "missing-root"},
		}, indexer.FileMatch{File: indexer.FileSummary{Path: "missing.go"}}, lineRange{start: 1, end: 1})
		Expect(err).To(MatchError(ContainSubstring("read file")))

		fixtureRoot := filepath.Join("..", "..", "internal", "indexer", "testdata", "fixtures", "polyglot")
		stdout := &bytes.Buffer{}
		Expect(runShowPackage(showPackageOptions{
			index: indexOptions{root: fixtureRoot, format: string(outputFormatText)},
			name:  "typescript",
		}, stdout)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Package: typescript"))

		err = runShowPackage(showPackageOptions{
			index: indexOptions{root: fixtureRoot},
			name:  "missing-package",
		}, &bytes.Buffer{})
		Expect(err).To(MatchError(ContainSubstring("was not found")))
	})

	It("covers run error handling and markdown cloning helpers", func() {
		stderr := &bytes.Buffer{}
		exitCode := run([]string{"--not-a-real-flag"}, &bytes.Buffer{}, stderr)
		Expect(exitCode).To(Equal(1))
		Expect(stderr.String()).To(ContainSubstring("unknown long flag"))

		document := markdownDocument{
			Title: "Doc",
			Sections: []markdownSection{{
				Heading: "Section",
				Items: []markdownListItem{{
					Text:     "**bold** [link](https://example.com) `code`",
					Children: []string{"_emphasis_"},
				}},
			}},
		}
		stdout := &bytes.Buffer{}
		Expect(renderMarkdownDocument(stdout, document)).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("# Doc"))
		Expect(stdout.String()).To(ContainSubstring("https://example.com"))

		stdout.Reset()
		Expect(writeMarkdownShowFile(stdout, showFileOutput{
			Path: "pkg/app.ts",
			Match: indexer.FileMatch{
				Package: "pkg",
				File:    indexer.FileSummary{Path: "pkg/app.ts", Language: "typescript", Bytes: 64},
			},
			Range: &showFileRangeView{
				RequestedStart: 1,
				RequestedEnd:   1,
				StartLine:      1,
				EndLine:        1,
				Lines:          []showFileLine{{Number: 1, Text: "const message = `hello`;"}},
			},
		})).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("`1:` const message = \\`hello\\`;"))

		Expect(cloneMarkdownNode(&ast.Text{})).To(BeAssignableToTypeOf(&ast.Text{}))
		Expect(cloneMarkdownNode(&ast.Code{})).To(BeAssignableToTypeOf(&ast.Code{}))
		Expect(cloneMarkdownNode(&ast.Emph{})).To(BeAssignableToTypeOf(&ast.Emph{}))
		Expect(cloneMarkdownNode(&ast.Strong{})).To(BeAssignableToTypeOf(&ast.Strong{}))
		Expect(cloneMarkdownNode(&ast.Link{})).To(BeAssignableToTypeOf(&ast.Link{}))
		Expect(cloneMarkdownNode(&ast.Hardbreak{})).To(BeAssignableToTypeOf(&ast.Hardbreak{}))
		Expect(cloneMarkdownNode(&ast.List{})).To(BeAssignableToTypeOf(&ast.Text{}))
	})
})

func skillInstallFixture() bundledskills.InstallInstructions {
	return bundledskills.InstallInstructions{
		InstallRoot:   "$CODEX_HOME/skills",
		CodexHomeHint: "Use the local Codex home.",
		Skills: []bundledskills.BundleInstallInstructions{
			{
				Name:        "scip-cli",
				Description: "CLI skill",
				InstallDir:  "$CODEX_HOME/skills/scip-cli",
				Steps:       []string{"Create the directory", "Write the files"},
				Files: []bundledskills.BundledFile{{
					RelativePath: "SKILL.md",
					InstallPath:  "$CODEX_HOME/skills/scip-cli/SKILL.md",
					Content:      "name: scip-cli\n",
				}},
			},
		},
	}
}
