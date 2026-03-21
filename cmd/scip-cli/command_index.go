package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/choria-io/fisk"

	"github.com/vjanelle/scip-cli/internal/indexer"
)

// indexOptions stores the shared CLI flags that map onto an indexing request.
type indexOptions struct {
	root                      string
	language                  string
	indexerName               string
	paths                     []string
	sampleLimit               int
	pageSize                  int
	pageToken                 string
	includeDeps               bool
	emitSCIP                  bool
	outputPath                string
	symbolicOnly              bool
	respectGitIgnore          bool
	maxFileBytes              int64
	includeHidden             bool
	summaryDetail             string
	maxSymbolsPerFile         int
	maxFiles                  int
	maxTokensApprox           int
	includeSpans              bool
	responseMode              string
	autoResponseMode          string
	autoHandleFileThreshold   int
	autoHandleSymbolThreshold int
	includeDebug              bool
	lspCommand                string
	lspArgs                   []string
	lspEnv                    []string
	lspInitOptions            string
	format                    string
	pretty                    bool
}

func init() {
	registerCLICommand(func(program *cliProgram, state *cliState) {
		indexCommand := program.app.Command("index", "Index a workspace and emit shaped results.")
		registerIndexFlags(indexCommand, &state.index)
		program.handlers["index"] = func(stdout io.Writer) error {
			return runIndex(state.index, stdout)
		}
	})
}

// registerCommonIndexFlags defines the shared workspace and output flags used
// by index, search, and show commands.
func registerCommonIndexFlags(command *fisk.CmdClause, options *indexOptions, includePathScope bool) {
	command.Flag("root", "Workspace root to index.").Default(".").StringVar(&options.root)
	command.Flag("language", "Optional language override.").StringVar(&options.language)
	command.Flag("indexer", "Optional backend override.").StringVar(&options.indexerName)
	if includePathScope {
		command.Flag("path", "Relative file or directory to include. Repeatable.").StringsVar(&options.paths)
	}
	command.Flag("sample-limit", "Maximum number of files to summarize.").IntVar(&options.sampleLimit)
	command.Flag("include-deps", "Include dependency metadata.").BoolVar(&options.includeDeps)
	command.Flag("emit-scip", "Write a SCIP snapshot to disk.").BoolVar(&options.emitSCIP)
	command.Flag("output-path", "Output path for the emitted SCIP snapshot.").StringVar(&options.outputPath)
	command.Flag("symbolic-only", "Return symbolic summaries without embedded bodies.").BoolVar(&options.symbolicOnly)
	command.Flag("respect-git-ignore", "Reserved for future filtering.").BoolVar(&options.respectGitIgnore)
	command.Flag("max-file-bytes", "Skip files larger than this byte threshold.").Int64Var(&options.maxFileBytes)
	command.Flag("include-hidden", "Include hidden files and directories.").BoolVar(&options.includeHidden)
	command.Flag("summary-detail", "Summary detail: minimal, normal, or deep.").StringVar(&options.summaryDetail)
	command.Flag("max-symbols-per-file", "Maximum inline symbols per file.").IntVar(&options.maxSymbolsPerFile)
	command.Flag("max-files", "Maximum inline files before trimming.").IntVar(&options.maxFiles)
	command.Flag("max-tokens-approx", "Approximate token budget for inline output.").IntVar(&options.maxTokensApprox)
	command.Flag("include-spans", "Include source spans in the response.").BoolVar(&options.includeSpans)
	command.Flag("response-mode", "Inline response mode: handles, compact, or detailed.").StringVar(&options.responseMode)
	command.Flag("auto-response-mode", "Enable or disable adaptive response shaping.").StringVar(&options.autoResponseMode)
	command.Flag("auto-handle-file-threshold", "File threshold for adaptive handles mode.").IntVar(&options.autoHandleFileThreshold)
	command.Flag("auto-handle-symbol-threshold", "Symbol threshold for adaptive handles mode.").IntVar(&options.autoHandleSymbolThreshold)
	command.Flag("include-debug", "Include debug details such as command lines.").BoolVar(&options.includeDebug)
	command.Flag("lsp-command", "Language server command for scip-lsp.").StringVar(&options.lspCommand)
	command.Flag("lsp-arg", "Argument passed to the configured language server. Repeatable.").StringsVar(&options.lspArgs)
	command.Flag("lsp-env", "Environment entry passed to the language server. Repeatable.").StringsVar(&options.lspEnv)
	command.Flag("lsp-init-options", "JSON object passed as LSP initialize options.").StringVar(&options.lspInitOptions)
	command.Flag("format", "Output format: json, markdown, or text.").Default(string(outputFormatJSON)).StringVar(&options.format)
	command.Flag("pretty", "Pretty-print JSON output.").BoolVar(&options.pretty)
}

// registerIndexFlags defines the extra pagination flags used by `index`.
func registerIndexFlags(command *fisk.CmdClause, options *indexOptions) {
	registerCommonIndexFlags(command, options, true)
	command.Flag("page-size", "Maximum number of files or dependencies to return.").IntVar(&options.pageSize)
	command.Flag("page-token", "Opaque pagination token from a prior run.").StringVar(&options.pageToken)
}

func runIndex(options indexOptions, stdout io.Writer) error {
	req, result, err := executeIndex(options, false)
	if err != nil {
		return err
	}

	result = indexer.PrepareResult(req, result)
	result, err = indexer.PaginateResult(result, req)
	if err != nil {
		return err
	}

	return renderIndexResult(stdout, options, result)
}

// executeIndex runs a fresh indexing pass and optionally expands the sampling
// budget for query-style commands that need the full in-memory result.
func executeIndex(options indexOptions, exhaustive bool) (indexer.IndexRequest, indexer.Result, error) {
	req, err := options.request()
	if err != nil {
		return indexer.IndexRequest{}, indexer.Result{}, err
	}

	// Search and show should inspect the full in-memory result rather than the
	// sampled summary used by default for one-shot index output.
	if exhaustive && req.SampleLimit == 0 {
		req.SampleLimit = exhaustiveSampleLimit
	}

	req, err = indexer.NormalizeRequest(req)
	if err != nil {
		return indexer.IndexRequest{}, indexer.Result{}, err
	}

	result, err := indexer.Dispatch(context.Background(), req)
	if err != nil {
		return indexer.IndexRequest{}, indexer.Result{}, err
	}

	return req, result, nil
}

// request translates CLI flag state into the shared indexer request type.
func (options indexOptions) request() (indexer.IndexRequest, error) {
	autoResponseMode, err := parseOptionalBool(options.autoResponseMode)
	if err != nil {
		return indexer.IndexRequest{}, err
	}

	initOptions, err := parseJSONObject(options.lspInitOptions)
	if err != nil {
		return indexer.IndexRequest{}, err
	}

	return indexer.IndexRequest{
		Root:                      options.root,
		Language:                  options.language,
		Indexer:                   options.indexerName,
		Paths:                     append([]string(nil), options.paths...),
		SampleLimit:               options.sampleLimit,
		PageSize:                  options.pageSize,
		PageToken:                 options.pageToken,
		IncludeDeps:               options.includeDeps,
		EmitSCIP:                  options.emitSCIP,
		OutputPath:                options.outputPath,
		SymbolicOnly:              options.symbolicOnly,
		RespectGitIgnore:          options.respectGitIgnore,
		MaxFileBytes:              options.maxFileBytes,
		IncludeHidden:             options.includeHidden,
		SummaryDetail:             options.summaryDetail,
		MaxSymbolsPerFile:         options.maxSymbolsPerFile,
		MaxFiles:                  options.maxFiles,
		MaxTokensApprox:           options.maxTokensApprox,
		IncludeSpans:              options.includeSpans,
		ResponseMode:              options.responseMode,
		AutoResponseMode:          autoResponseMode,
		AutoHandleFileThreshold:   options.autoHandleFileThreshold,
		AutoHandleSymbolThreshold: options.autoHandleSymbolThreshold,
		IncludeDebug:              options.includeDebug,
		LSPCommand:                options.lspCommand,
		LSPArgs:                   append([]string(nil), options.lspArgs...),
		LSPEnv:                    append([]string(nil), options.lspEnv...),
		LSPInitOptions:            initOptions,
	}, nil
}

// parseOptionalBool parses the string-backed adaptive mode flag used by fisk.
func parseOptionalBool(raw string) (*bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return nil, nil
	}

	switch normalized {
	case "true", "1", "yes", "on":
		value := true
		return &value, nil
	case "false", "0", "no", "off":
		value := false
		return &value, nil
	default:
		return nil, fmt.Errorf("invalid boolean value %q", raw)
	}
}

// parseJSONObject decodes the raw JSON passed to `--lsp-init-options`.
func parseJSONObject(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	decoded := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, fmt.Errorf("parse lsp init options: %w", err)
	}

	return decoded, nil
}
