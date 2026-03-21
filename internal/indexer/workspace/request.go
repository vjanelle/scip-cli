package workspace

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/vjanelle/scip-cli/internal/indexer/model"
)

const (
	// These defaults stay here so request normalization rules live with workspace request shaping.
	DefaultSampleLimit               = 24
	DefaultPageSize                  = 50
	DefaultMaxFileBytes              = 512 * 1024
	DefaultMaxFiles                  = 12
	DefaultMaxSymbols                = 8
	DefaultMaxTokens                 = 2400
	DefaultAutoHandleFileThreshold   = 120
	DefaultAutoHandleSymbolThreshold = 3000
)

// NormalizeRequest validates required fields and applies stable request defaults.
func NormalizeRequest(req model.IndexRequest) (model.IndexRequest, error) {
	if strings.TrimSpace(req.Root) == "" {
		return model.IndexRequest{}, errors.New("root is required")
	}

	root, err := filepath.Abs(req.Root)
	if err != nil {
		return model.IndexRequest{}, err
	}

	req.Root = root
	req.Indexer = NormalizeIndexer(req.Indexer)
	req.Language = NormalizeLanguage(req.Language)
	req.LSPCommand = strings.TrimSpace(req.LSPCommand)
	if req.SampleLimit <= 0 {
		req.SampleLimit = DefaultSampleLimit
	}
	if req.PageSize <= 0 {
		req.PageSize = DefaultPageSize
	}
	if !req.IncludeDeps {
		req.IncludeDeps = true
	}
	if req.MaxFileBytes <= 0 {
		req.MaxFileBytes = DefaultMaxFileBytes
	}
	if req.MaxFiles <= 0 {
		req.MaxFiles = DefaultMaxFiles
	}
	if req.MaxSymbolsPerFile <= 0 {
		req.MaxSymbolsPerFile = DefaultMaxSymbols
	}
	if req.MaxTokensApprox <= 0 {
		req.MaxTokensApprox = DefaultMaxTokens
	}
	if req.ResponseMode == "" {
		req.ResponseMode = "compact"
	}
	if req.AutoResponseMode == nil {
		value := true
		req.AutoResponseMode = &value
	}
	if req.AutoHandleFileThreshold <= 0 {
		req.AutoHandleFileThreshold = DefaultAutoHandleFileThreshold
	}
	if req.AutoHandleSymbolThreshold <= 0 {
		req.AutoHandleSymbolThreshold = DefaultAutoHandleSymbolThreshold
	}
	if req.SummaryDetail == "" {
		req.SummaryDetail = "normal"
	}
	req.ResponseMode = strings.ToLower(strings.TrimSpace(req.ResponseMode))
	switch req.ResponseMode {
	case "handles", "compact", "detailed":
	default:
		req.ResponseMode = "compact"
	}
	req.SummaryDetail = strings.ToLower(strings.TrimSpace(req.SummaryDetail))
	switch req.SummaryDetail {
	case "minimal", "normal", "deep":
	default:
		req.SummaryDetail = "normal"
	}
	if req.ResponseMode == "handles" && req.SummaryDetail == "deep" {
		req.SummaryDetail = "normal"
	}
	if req.SummaryDetail == "deep" {
		req.IncludeSpans = true
	}

	cleanedPaths := make([]string, 0, len(req.Paths))
	for _, path := range req.Paths {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "." || path == "" {
			continue
		}
		cleanedPaths = append(cleanedPaths, path)
	}
	req.Paths = cleanedPaths
	req.LSPArgs = CleanedArgs(req.LSPArgs)
	req.LSPEnv = CleanedArgs(req.LSPEnv)

	return req, nil
}

// NormalizeLanguage canonicalizes common language aliases used by clients.
func NormalizeLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "golang":
		return "go"
	case "javascript", "node", "nodejs":
		return "javascript"
	case "typescript", "ts":
		return "typescript"
	case "py":
		return "python"
	case "rs":
		return "rust"
	default:
		return strings.ToLower(strings.TrimSpace(language))
	}
}

// NormalizeIndexer maps legacy or user-friendly backend names to stable internal IDs.
func NormalizeIndexer(indexer string) string {
	switch strings.ToLower(strings.TrimSpace(indexer)) {
	case "", "auto":
		return ""
	case "go", "scip-go":
		return "go"
	case "tree-sitter", "symbolic-fallback":
		return "tree-sitter"
	case "lsp", "scip-lsp":
		return "scip-lsp"
	default:
		return strings.ToLower(strings.TrimSpace(indexer))
	}
}

// CleanedArgs drops blank entries from repeated CLI-provided argument slices.
func CleanedArgs(values []string) []string {
	args := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		args = append(args, value)
	}
	return args
}
