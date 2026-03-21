package main

import (
	"fmt"
	"io"

	"github.com/choria-io/fisk"

	"github.com/vjanelle/scip-cli/internal/indexer"
)

// showFileOptions combines an index request with an exact file path lookup.
type showFileOptions struct {
	index indexOptions
	path  string
}

// showPackageOptions combines an index request with an exact package lookup.
type showPackageOptions struct {
	index indexOptions
	name  string
}

// showFileOutput is the structured payload emitted by `show file`.
type showFileOutput struct {
	Path  string            `json:"path"`
	Match indexer.FileMatch `json:"match"`
}

// showPackageOutput is the structured payload emitted by `show package`.
type showPackageOutput struct {
	Name  string               `json:"name"`
	Match indexer.PackageMatch `json:"match"`
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
}

// registerShowPackageFlags adds the exact package lookup flag used by `show package`.
func registerShowPackageFlags(command *fisk.CmdClause, options *showPackageOptions) {
	registerCommonIndexFlags(command, &options.index, true)
	command.Flag("name", "Exact package or directory name to show.").Required().StringVar(&options.name)
}

func runShowFile(options showFileOptions, stdout io.Writer) error {
	_, result, err := executeIndex(options.index, true)
	if err != nil {
		return err
	}

	match, ok := indexer.FindFile(result, options.path)
	if !ok {
		return fmt.Errorf("file %q was not found in the indexed result", options.path)
	}

	return renderShowFileOutput(stdout, options.index, showFileOutput{
		Path:  options.path,
		Match: match,
	})
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
