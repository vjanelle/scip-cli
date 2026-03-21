package main

import (
	"fmt"
	"io"

	"github.com/choria-io/fisk"

	"github.com/vjanelle/scip-cli/internal/indexer"
)

// searchOptions combines an index request with a query string for search commands.
type searchOptions struct {
	index indexOptions
	query string
}

// searchFilesOutput is the structured payload emitted by `search files`.
type searchFilesOutput struct {
	Query        string              `json:"query"`
	TotalMatches int                 `json:"totalMatches"`
	Files        []indexer.FileMatch `json:"files"`
}

// searchSymbolsOutput is the structured payload emitted by `search symbols`.
type searchSymbolsOutput struct {
	Query        string           `json:"query"`
	TotalMatches int              `json:"totalMatches"`
	Symbols      []indexer.Symbol `json:"symbols"`
}

// searchWarningsOutput is the structured payload emitted by `search warnings`.
type searchWarningsOutput struct {
	Query        string   `json:"query"`
	TotalMatches int      `json:"totalMatches"`
	Warnings     []string `json:"warnings"`
}

func init() {
	registerCLICommand(func(program *cliProgram, state *cliState) {
		searchCommand := program.app.Command("search", "Search an indexed workspace.")

		filesCommand := searchCommand.Command("files", "Search file paths and file-level symbol context.")
		registerSearchFlags(filesCommand, &state.searchFiles)
		program.handlers["search files"] = func(stdout io.Writer) error {
			return runSearchFiles(state.searchFiles, stdout)
		}

		symbolsCommand := searchCommand.Command("symbols", "Search symbols across indexed files.")
		registerSearchFlags(symbolsCommand, &state.searchSymbols)
		program.handlers["search symbols"] = func(stdout io.Writer) error {
			return runSearchSymbols(state.searchSymbols, stdout)
		}

		warningsCommand := searchCommand.Command("warnings", "Search warning text emitted during indexing.")
		registerSearchFlags(warningsCommand, &state.searchWarnings)
		program.handlers["search warnings"] = func(stdout io.Writer) error {
			return runSearchWarnings(state.searchWarnings, stdout)
		}
	})
}

// registerSearchFlags adds the query flag used by the search subcommands.
func registerSearchFlags(command *fisk.CmdClause, options *searchOptions) {
	registerCommonIndexFlags(command, &options.index, true)
	command.Flag("query", "Case-insensitive query text.").Required().StringVar(&options.query)
}

func runSearchFiles(options searchOptions, stdout io.Writer) error {
	_, result, err := executeIndex(options.index, true)
	if err != nil {
		return err
	}

	output := searchFilesOutput{
		Query: options.query,
		Files: indexer.SearchFiles(result, options.query),
	}
	output.TotalMatches = len(output.Files)

	return renderSearchFilesOutput(stdout, options.index, output)
}

func runSearchSymbols(options searchOptions, stdout io.Writer) error {
	_, result, err := executeIndex(options.index, true)
	if err != nil {
		return err
	}

	output := searchSymbolsOutput{
		Query:   options.query,
		Symbols: indexer.SearchSymbols(result, options.query),
	}
	output.TotalMatches = len(output.Symbols)

	return renderSearchSymbolsOutput(stdout, options.index, output)
}

func runSearchWarnings(options searchOptions, stdout io.Writer) error {
	_, result, err := executeIndex(options.index, true)
	if err != nil {
		return err
	}

	output := searchWarningsOutput{
		Query:    options.query,
		Warnings: indexer.SearchWarnings(result, options.query),
	}
	output.TotalMatches = len(output.Warnings)

	return renderSearchWarningsOutput(stdout, options.index, output)
}

func renderSearchFilesOutput(stdout io.Writer, options indexOptions, output searchFilesOutput) error {
	format, err := normalizeOutputFormat(options.format)
	if err != nil {
		return err
	}

	switch format {
	case outputFormatJSON:
		return writeJSON(stdout, output, options.pretty)
	case outputFormatMarkdown:
		return writeMarkdownSearchFiles(stdout, output)
	case outputFormatText:
		return writeTextSearchFiles(stdout, output)
	default:
		return fmt.Errorf("unsupported output format %q", options.format)
	}
}

func renderSearchSymbolsOutput(stdout io.Writer, options indexOptions, output searchSymbolsOutput) error {
	format, err := normalizeOutputFormat(options.format)
	if err != nil {
		return err
	}

	switch format {
	case outputFormatJSON:
		return writeJSON(stdout, output, options.pretty)
	case outputFormatMarkdown:
		return writeMarkdownSearchSymbols(stdout, output)
	case outputFormatText:
		return writeTextSearchSymbols(stdout, output)
	default:
		return fmt.Errorf("unsupported output format %q", options.format)
	}
}

func renderSearchWarningsOutput(stdout io.Writer, options indexOptions, output searchWarningsOutput) error {
	format, err := normalizeOutputFormat(options.format)
	if err != nil {
		return err
	}

	switch format {
	case outputFormatJSON:
		return writeJSON(stdout, output, options.pretty)
	case outputFormatMarkdown:
		return writeMarkdownSearchWarnings(stdout, output)
	case outputFormatText:
		return writeTextSearchWarnings(stdout, output)
	default:
		return fmt.Errorf("unsupported output format %q", options.format)
	}
}
