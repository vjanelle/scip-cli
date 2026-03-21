package indexer

import (
	"io/fs"

	"github.com/vjanelle/scip-cli/internal/indexer/workspace"
)

var ignoredDirs = workspace.IgnoredDirs

var languageByExtension = workspace.LanguageByExtension

// CollectFiles walks the requested scope and returns candidate source files.
// Warnings are returned for non-fatal issues such as missing subpaths.
func CollectFiles(req IndexRequest) ([]string, []string, error) {
	return workspace.CollectFiles(req)
}

func shouldSkipDir(req IndexRequest, name string) bool {
	return workspace.ShouldSkipDir(req, name)
}

func includePath(req IndexRequest, rel string, info fs.FileInfo) bool {
	return workspace.IncludePath(req, rel, info)
}

// DetectLanguage maps file extensions to the indexer's supported language IDs.
func DetectLanguage(path string) string {
	return workspace.DetectLanguage(path)
}

// SupportedLanguages returns the normalized language IDs supported by the indexer.
func SupportedLanguages() []string {
	return workspace.SupportedLanguages()
}
