package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/choria-io/fisk"
)

const (
	exhaustiveSampleLimit = 1_000_000
	version               = "dev"
)

type cliExit struct {
	code int
}

func (exit cliExit) Error() string {
	return fmt.Sprintf("exit %d", exit.code)
}

type outputFormat string

const (
	outputFormatJSON     outputFormat = "json"
	outputFormatMarkdown outputFormat = "markdown"
	outputFormatText     outputFormat = "text"
)

type cliState struct {
	index          indexOptions
	searchFiles    searchOptions
	searchSymbols  searchOptions
	searchWarnings searchOptions
	showFile       showFileOptions
	showPackage    showPackageOptions
	skillInstall   skillInstallOptions
}

// defaultIndexOptions returns the CLI defaults before request normalization.
func defaultIndexOptions() indexOptions {
	return indexOptions{
		includeDeps: true,
		format:      string(outputFormatJSON),
		pretty:      true,
	}
}

func newCLIState() cliState {
	return cliState{
		index:          defaultIndexOptions(),
		searchFiles:    searchOptions{index: defaultIndexOptions()},
		searchSymbols:  searchOptions{index: defaultIndexOptions()},
		searchWarnings: searchOptions{index: defaultIndexOptions()},
		showFile:       showFileOptions{index: defaultIndexOptions()},
		showPackage:    showPackageOptions{index: defaultIndexOptions()},
		skillInstall: skillInstallOptions{
			format: string(outputFormatJSON),
			pretty: true,
		},
	}
}

// cliProgram keeps parser registration and command execution together so
// command-focused files can self-register during init without reaching back
// into main.go.
type cliProgram struct {
	app      *fisk.Application
	handlers map[string]commandHandler
}

type commandHandler func(stdout io.Writer) error
type commandRegistrar func(program *cliProgram, state *cliState)

var commandRegistrars []commandRegistrar

// registerCLICommand records one command module's parser wiring. Each command
// file calls this from init so the root CLI stays small and declarative.
func registerCLICommand(register commandRegistrar) {
	commandRegistrars = append(commandRegistrars, register)
}

func newCLIProgram(stdout, stderr io.Writer, state *cliState) cliProgram {
	app := fisk.New("scip-cli", "A source indexing CLI.")
	app.Version(version)
	app.UsageWriter(stdout)
	app.ErrorWriter(stderr)
	app.Terminate(func(code int) {
		panic(cliExit{code: code})
	})

	// Command files populate both the parser tree and the runtime dispatch map
	// through the same registration hook.
	program := cliProgram{
		app:      app,
		handlers: map[string]commandHandler{},
	}
	for _, register := range commandRegistrars {
		register(&program, state)
	}

	return program
}

// normalizeArgs preserves the `scip-cli --root .` shorthand by routing a
// top-level flag invocation to the `index` command.
func normalizeArgs(args []string) []string {
	if len(args) == 0 {
		return []string{"--help"}
	}

	first := strings.TrimSpace(args[0])
	if first == "" {
		return []string{"--help"}
	}
	if strings.HasPrefix(first, "-") && first != "--help" && first != "-h" && first != "--version" {
		return append([]string{"index"}, args...)
	}

	return args
}
