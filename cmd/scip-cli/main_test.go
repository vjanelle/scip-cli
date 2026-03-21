package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CLI", func() {
	// The polyglot fixture lets the CLI specs exercise multiple language paths
	// without depending on the current repository layout.
	fixtureRoot := filepath.Join("..", "..", "internal", "indexer", "testdata", "fixtures", "polyglot")

	It("emits JSON output for index runs", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"index", "--root", fixtureRoot, "--language", "python", "--no-pretty"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())

		var decoded struct {
			FilesIndexed  int `json:"filesIndexed"`
			ReturnedFiles int `json:"returnedFiles"`
		}
		// JSON assertions keep the test focused on the structured contract
		// instead of incidental formatting details.
		Expect(json.Unmarshal(stdout.Bytes(), &decoded)).To(Succeed(), "output: %s", stdout.String())
		Expect(decoded.FilesIndexed).To(Equal(1))
		Expect(decoded.ReturnedFiles).To(Equal(1))
	})

	It("supports top-level index shorthand with text output", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"--root", fixtureRoot, "--language", "typescript", "--format", "text"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())
		Expect(stdout.String()).To(ContainSubstring("Indexer:"))
		Expect(stdout.String()).To(ContainSubstring("Top files:"))
	})

	It("emits LLM-friendly markdown output", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"index", "--root", fixtureRoot, "--language", "javascript", "--format", "markdown"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())
		Expect(stdout.String()).To(ContainSubstring("# Index Result"))
		Expect(stdout.String()).To(ContainSubstring("## Summary"))
		Expect(stdout.String()).To(ContainSubstring("## Files"))
	})

	It("searches files with markdown output", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"search", "files", "--root", fixtureRoot, "--query", "app", "--format", "markdown"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())
		Expect(stdout.String()).To(ContainSubstring("# File Search"))
		Expect(stdout.String()).To(ContainSubstring("## Files"))
	})

	It("searches symbols with JSON output", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"search", "symbols", "--root", fixtureRoot, "--query", "greet", "--no-pretty"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())

		var decoded struct {
			TotalMatches int `json:"totalMatches"`
		}
		// The exact symbol set can evolve, but a positive match count proves the
		// query path and JSON shaping are wired together correctly.
		Expect(json.Unmarshal(stdout.Bytes(), &decoded)).To(Succeed(), "output: %s", stdout.String())
		Expect(decoded.TotalMatches).To(BeNumerically(">", 0))
	})

	It("shows one file with markdown output", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"show", "file", "--root", fixtureRoot, "--path", "python/app.py", "--format", "markdown"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())
		Expect(stdout.String()).To(ContainSubstring("# File Detail"))
		Expect(stdout.String()).To(ContainSubstring("python/app.py"))
	})

	It("shows one file range with text output", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"show", "file", "--root", fixtureRoot, "--path", "typescript/app.ts", "--range", "3-4", "--format", "text"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())
		Expect(stdout.String()).To(ContainSubstring("Requested range: 3-4"))
		Expect(stdout.String()).To(ContainSubstring("Returned range: 3-4"))
		Expect(stdout.String()).To(ContainSubstring("3: function greet(): string {"))
		Expect(stdout.String()).To(ContainSubstring("4:   return \"hello\";"))
		Expect(stdout.String()).To(ContainSubstring("- greet [function] 3-5"))
		Expect(stdout.String()).NotTo(ContainSubstring("- Greeter [type] 1-1"))
	})

	It("shows one file range with markdown output", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"show", "file", "--root", fixtureRoot, "--path", "typescript/app.ts", "--range", "1-2", "--format", "markdown"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())
		Expect(stdout.String()).To(ContainSubstring("# File Detail"))
		Expect(stdout.String()).To(ContainSubstring("## Range"))
		Expect(stdout.String()).To(ContainSubstring("## Source"))
		Expect(stdout.String()).To(ContainSubstring("## Symbols"))
		Expect(stdout.String()).To(ContainSubstring("`1:` interface Greeter {}"))
	})

	It("shows one file range with JSON output", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"show", "file", "--root", fixtureRoot, "--path", "typescript/app.ts", "--range", "1-3", "--no-pretty"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())

		var decoded struct {
			Path  string `json:"path"`
			Range struct {
				RequestedStart int `json:"requestedStart"`
				RequestedEnd   int `json:"requestedEnd"`
				StartLine      int `json:"startLine"`
				EndLine        int `json:"endLine"`
				Lines          []struct {
					Number int    `json:"number"`
					Text   string `json:"text"`
				} `json:"lines"`
				Symbols []struct {
					Name      string `json:"name"`
					StartLine int    `json:"startLine"`
					EndLine   int    `json:"endLine"`
				} `json:"symbols"`
			} `json:"range"`
		}
		Expect(json.Unmarshal(stdout.Bytes(), &decoded)).To(Succeed(), "output: %s", stdout.String())
		Expect(decoded.Path).To(Equal("typescript/app.ts"))
		Expect(decoded.Range.RequestedStart).To(Equal(1))
		Expect(decoded.Range.RequestedEnd).To(Equal(3))
		Expect(decoded.Range.StartLine).To(Equal(1))
		Expect(decoded.Range.EndLine).To(Equal(3))
		Expect(decoded.Range.Lines).To(HaveLen(3))
		Expect(decoded.Range.Symbols).To(HaveLen(2))
		Expect(decoded.Range.Symbols[0].Name).To(Equal("Greeter"))
		Expect(decoded.Range.Symbols[1].Name).To(Equal("greet"))
	})

	It("omits range payload when show file range is not requested", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"show", "file", "--root", fixtureRoot, "--path", "typescript/app.ts", "--no-pretty"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())

		var decoded struct {
			Range any `json:"range"`
		}
		Expect(json.Unmarshal(stdout.Bytes(), &decoded)).To(Succeed(), "output: %s", stdout.String())
		Expect(decoded.Range).To(BeNil())
	})

	It("rejects invalid show file ranges", func() {
		for _, raw := range []string{"abc", "4", "4-2", "0-2", "1-two", "1-2-3"} {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			exitCode := run([]string{"show", "file", "--root", fixtureRoot, "--path", "typescript/app.ts", "--range", raw}, stdout, stderr)
			Expect(exitCode).To(Equal(1), "range %s stderr: %s", raw, stderr.String())
			Expect(stdout.String()).To(BeEmpty(), "range %s stdout: %s", raw, stdout.String())
			Expect(stderr.String()).To(ContainSubstring("parse range"), "range %s", raw)
		}
	})

	It("clamps show file ranges at EOF", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"show", "file", "--root", fixtureRoot, "--path", "typescript/app.ts", "--range", "4-10", "--format", "text"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())
		Expect(stdout.String()).To(ContainSubstring("Requested range: 4-10"))
		Expect(stdout.String()).To(ContainSubstring("Returned range: 4-5"))
		Expect(stdout.String()).To(ContainSubstring("5: }"))
	})

	It("errors when a show file range starts past EOF", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"show", "file", "--root", fixtureRoot, "--path", "typescript/app.ts", "--range", "6-10", "--format", "text"}, stdout, stderr)
		Expect(exitCode).To(Equal(1))
		Expect(stdout.String()).To(BeEmpty())
		Expect(stderr.String()).To(ContainSubstring("starts past end of file"))
	})

	It("returns an empty symbol list when a show file range has no overlaps", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"show", "file", "--root", fixtureRoot, "--path", "typescript/app.ts", "--range", "2-2", "--format", "text"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())
		Expect(stdout.String()).To(ContainSubstring("2: "))
		Expect(stdout.String()).To(ContainSubstring("Symbols:"))
		Expect(stdout.String()).NotTo(ContainSubstring("- greet [function]"))
		Expect(stdout.String()).NotTo(ContainSubstring("- Greeter [type]"))
	})

	It("documents the show file range flag in help output", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"show", "file", "--help"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())
		Expect(stdout.String()).To(ContainSubstring("--range"))
		Expect(stdout.String()).To(ContainSubstring("START-END"))
	})

	It("shows one package with text output", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"show", "package", "--root", fixtureRoot, "--name", "python", "--format", "text"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())
		Expect(stdout.String()).To(ContainSubstring("Package: python"))
	})

	It("emits bundled skill install instructions in markdown", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"skills", "install-instructions", "--format", "markdown"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())
		Expect(stdout.String()).To(ContainSubstring("# Skill Install Instructions"))
		Expect(stdout.String()).To(ContainSubstring("`scip-cli`"))
		Expect(stdout.String()).To(ContainSubstring("$CODEX_HOME/skills/scip-cli/SKILL.md"))
		Expect(stdout.String()).To(ContainSubstring("name: scip-cli"))
	})

	It("emits bundled skill install instructions in JSON", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"skills", "install-instructions", "--name", "scip-cli", "--no-pretty"}, stdout, stderr)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr.String())

		var decoded struct {
			Skills []struct {
				Name  string `json:"name"`
				Files []struct {
					RelativePath string `json:"relativePath"`
				} `json:"files"`
			} `json:"skills"`
		}
		// The embedded skill output should stay machine-readable so agents can
		// install the bundled files without scraping Markdown.
		Expect(json.Unmarshal(stdout.Bytes(), &decoded)).To(Succeed(), "output: %s", stdout.String())
		Expect(decoded.Skills).To(HaveLen(1))
		Expect(decoded.Skills[0].Name).To(Equal("scip-cli"))
		Expect(decoded.Skills[0].Files).To(HaveLen(2))
	})

	It("rejects invalid LSP init options", func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		exitCode := run([]string{"index", "--root", ".", "--lsp-init-options", "{nope}"}, stdout, stderr)
		Expect(exitCode).To(Equal(1))
		Expect(stdout.String()).To(BeEmpty())
		Expect(stderr.String()).To(ContainSubstring("parse lsp init options"))
	})
})
