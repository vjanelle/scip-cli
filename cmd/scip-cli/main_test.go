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
