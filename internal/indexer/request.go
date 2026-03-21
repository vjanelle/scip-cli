package indexer

import "github.com/vjanelle/scip-cli/internal/indexer/workspace"

const (
	defaultSampleLimit               = workspace.DefaultSampleLimit
	defaultPageSize                  = workspace.DefaultPageSize
	defaultMaxFileBytes              = workspace.DefaultMaxFileBytes
	defaultMaxFiles                  = workspace.DefaultMaxFiles
	defaultMaxSymbols                = workspace.DefaultMaxSymbols
	defaultMaxTokens                 = workspace.DefaultMaxTokens
	DefaultAutoHandleFileThreshold   = workspace.DefaultAutoHandleFileThreshold
	DefaultAutoHandleSymbolThreshold = workspace.DefaultAutoHandleSymbolThreshold
)

// NormalizeRequest validates required fields and applies stable request defaults.
func NormalizeRequest(req IndexRequest) (IndexRequest, error) {
	return workspace.NormalizeRequest(req)
}

// normalizeLanguage remains here as a compatibility shim for package-local callers and tests.
func normalizeLanguage(language string) string {
	return workspace.NormalizeLanguage(language)
}

func normalizeIndexer(indexer string) string {
	return workspace.NormalizeIndexer(indexer)
}

func cleanedArgs(values []string) []string {
	return workspace.CleanedArgs(values)
}
