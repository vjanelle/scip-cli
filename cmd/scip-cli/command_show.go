package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/choria-io/fisk"

	"github.com/vjanelle/scip-cli/internal/indexer"
)

// showFileOptions combines an index request with an exact file path lookup.
type showFileOptions struct {
	index     indexOptions
	path      string
	lineRange string
}

// showPackageOptions combines an index request with an exact package lookup.
type showPackageOptions struct {
	index indexOptions
	name  string
}

// showFileOutput is the structured payload emitted by `show file`.
type showFileOutput struct {
	Path  string             `json:"path"`
	Match indexer.FileMatch  `json:"match"`
	Range *showFileRangeView `json:"range,omitempty"`
}

// showPackageOutput is the structured payload emitted by `show package`.
type showPackageOutput struct {
	Name  string               `json:"name"`
	Match indexer.PackageMatch `json:"match"`
}

// showFileRangeView carries the source snippet and overlap-filtered symbols for
// `show file --range`, while leaving the existing whole-file output unchanged.
type showFileRangeView struct {
	RequestedStart int              `json:"requestedStart"`
	RequestedEnd   int              `json:"requestedEnd"`
	StartLine      int              `json:"startLine"`
	EndLine        int              `json:"endLine"`
	Lines          []showFileLine   `json:"lines"`
	Symbols        []indexer.Symbol `json:"symbols,omitempty"`
}

// showFileLine stores one numbered source line from the requested snippet.
type showFileLine struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

type lineRange struct {
	start int
	end   int
}

func init() {
	registerCLICommand(func(program *cliProgram, state *cliState) {
		showCommand := program.app.Command("show", "Show targeted detail for one indexed entity.")

		fileCommand := showCommand.Command("file", "Show one file by exact relative path.")
		registerShowFileFlags(fileCommand, &state.showFile)
		program.handlers["show file"] = func(stdout io.Writer) error {
			return runShowFile(state.showFile, stdout)
		}

		packageCommand := showCommand.Command("package", "Show one package or directory slice by exact name.")
		registerShowPackageFlags(packageCommand, &state.showPackage)
		program.handlers["show package"] = func(stdout io.Writer) error {
			return runShowPackage(state.showPackage, stdout)
		}
	})
}

// registerShowFileFlags adds the exact file lookup flag used by `show file`.
func registerShowFileFlags(command *fisk.CmdClause, options *showFileOptions) {
	registerCommonIndexFlags(command, &options.index, false)
	command.Flag("path", "Exact relative file path to show.").Required().StringVar(&options.path)
	command.Flag("range", "Inclusive 1-based line range to show, formatted as START-END.").StringVar(&options.lineRange)
}

// registerShowPackageFlags adds the exact package lookup flag used by `show package`.
func registerShowPackageFlags(command *fisk.CmdClause, options *showPackageOptions) {
	registerCommonIndexFlags(command, &options.index, true)
	command.Flag("name", "Exact package or directory name to show.").Required().StringVar(&options.name)
}

func runShowFile(options showFileOptions, stdout io.Writer) error {
	selectedRange, err := parseShowFileRange(options.lineRange)
	if err != nil {
		return err
	}

	_, result, err := executeIndex(options.index, true)
	if err != nil {
		return err
	}

	match, ok := indexer.FindFile(result, options.path)
	if !ok {
		return fmt.Errorf("file %q was not found in the indexed result", options.path)
	}

	output := showFileOutput{
		Path:  options.path,
		Match: match,
	}
	if selectedRange != nil {
		output.Range, err = buildShowFileRangeView(options, match, *selectedRange)
		if err != nil {
			return err
		}
	}

	return renderShowFileOutput(stdout, options.index, output)
}

// parseShowFileRange validates the CLI-facing START-END syntax before any
// expensive indexing or rendering work happens.
func parseShowFileRange(raw string) (*lineRange, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("parse range %q: expected START-END", raw)
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("parse range %q: start line must be a positive integer", raw)
	}
	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("parse range %q: end line must be a positive integer", raw)
	}
	if start <= 0 || end <= 0 {
		return nil, fmt.Errorf("parse range %q: line numbers must be positive", raw)
	}
	if start > end {
		return nil, fmt.Errorf("parse range %q: start line must be less than or equal to end line", raw)
	}

	return &lineRange{start: start, end: end}, nil
}

// buildShowFileRangeView reads the requested file from disk so the range output
// reflects the current workspace text rather than only the indexed metadata.
func buildShowFileRangeView(options showFileOptions, match indexer.FileMatch, selected lineRange) (*showFileRangeView, error) {
	absolute := filepath.Join(options.index.root, filepath.FromSlash(match.File.Path))
	payload, err := os.ReadFile(absolute)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", match.File.Path, err)
	}

	lines := splitSourceLines(string(payload))
	if selected.start > len(lines) {
		return nil, fmt.Errorf("range %d-%d starts past end of file %q (%d lines)", selected.start, selected.end, match.File.Path, len(lines))
	}

	effectiveEnd := min(selected.end, len(lines))
	snippet := make([]showFileLine, 0, effectiveEnd-selected.start+1)
	for number := selected.start; number <= effectiveEnd; number++ {
		snippet = append(snippet, showFileLine{
			Number: number,
			Text:   lines[number-1],
		})
	}

	filteredSymbols := make([]indexer.Symbol, 0, len(match.File.Symbols))
	for _, symbol := range match.File.Symbols {
		if symbol.StartLine <= selected.end && symbol.EndLine >= selected.start {
			filteredSymbols = append(filteredSymbols, symbol)
		}
	}

	return &showFileRangeView{
		RequestedStart: selected.start,
		RequestedEnd:   selected.end,
		StartLine:      selected.start,
		EndLine:        effectiveEnd,
		Lines:          snippet,
		Symbols:        filteredSymbols,
	}, nil
}

func runShowPackage(options showPackageOptions, stdout io.Writer) error {
	_, result, err := executeIndex(options.index, true)
	if err != nil {
		return err
	}

	match, ok := indexer.FindPackage(result, options.name)
	if !ok {
		return fmt.Errorf("package %q was not found in the indexed result", options.name)
	}

	return renderShowPackageOutput(stdout, options.index, showPackageOutput{
		Name:  options.name,
		Match: match,
	})
}

func renderShowFileOutput(stdout io.Writer, options indexOptions, output showFileOutput) error {
	format, err := normalizeOutputFormat(options.format)
	if err != nil {
		return err
	}

	switch format {
	case outputFormatJSON:
		return writeJSON(stdout, output, options.pretty)
	case outputFormatMarkdown:
		return writeMarkdownShowFile(stdout, output)
	case outputFormatText:
		return writeTextShowFile(stdout, output)
	default:
		return fmt.Errorf("unsupported output format %q", options.format)
	}
}

func splitSourceLines(source string) []string {
	scanner := bufio.NewScanner(strings.NewReader(source))
	// The CLI already holds the whole file in memory, so grow the scanner limit
	// to the file size and let ScanLines normalize LF and CRLF terminators.
	scanner.Buffer(make([]byte, 0, min(len(source), 64*1024)), max(len(source), 64*1024))

	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		// Scanning an in-memory string should not fail in practice, so fall back
		// to the raw source as one line instead of dropping content unexpectedly.
		return []string{source}
	}

	// bufio.Scanner treats a trailing newline as a line terminator rather than
	// an extra empty line, which matches how humans count source lines.
	return lines
}

func renderShowPackageOutput(stdout io.Writer, options indexOptions, output showPackageOutput) error {
	format, err := normalizeOutputFormat(options.format)
	if err != nil {
		return err
	}

	switch format {
	case outputFormatJSON:
		return writeJSON(stdout, output, options.pretty)
	case outputFormatMarkdown:
		return writeMarkdownShowPackage(stdout, output)
	case outputFormatText:
		return writeTextShowPackage(stdout, output)
	default:
		return fmt.Errorf("unsupported output format %q", options.format)
	}
}
