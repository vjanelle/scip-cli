package queries

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/vjanelle/scip-cli/internal/indexer/model"
	"github.com/vjanelle/scip-cli/internal/indexer/workspace"
)

// FileMatch is a file-level query hit enriched with its logical package label.
type FileMatch struct {
	Package string            `json:"package"`
	File    model.FileSummary `json:"file"`
}

// PackageMatch is a package-level query hit with the files that belong to it.
type PackageMatch struct {
	Package model.PackageSummary `json:"package"`
	Files   []model.FileSummary  `json:"files"`
}

// SearchFiles matches files by path and symbol name using a case-insensitive contains query.
func SearchFiles(result model.Result, query string) []FileMatch {
	normalized := NormalizeQuery(query)
	if normalized == "" {
		return nil
	}

	matches := make([]FileMatch, 0, len(result.FileSummaries))
	for _, file := range result.FileSummaries {
		if FileMatchesQuery(file, normalized) {
			matches = append(matches, FileMatch{
				Package: workspace.PackageNameForPath(file.Path),
				File:    file,
			})
		}
	}

	slices.SortFunc(matches, func(left, right FileMatch) int {
		return strings.Compare(filepath.ToSlash(left.File.Path), filepath.ToSlash(right.File.Path))
	})

	return matches
}

// SearchSymbols returns matching symbols across all indexed files.
func SearchSymbols(result model.Result, query string) []model.Symbol {
	normalized := NormalizeQuery(query)
	if normalized == "" {
		return nil
	}

	matches := make([]model.Symbol, 0, 16)
	for _, file := range result.FileSummaries {
		for _, symbol := range file.Symbols {
			if MatchesQuery(symbol.Name, normalized) || MatchesQuery(symbol.Kind, normalized) || MatchesQuery(symbol.Path, normalized) {
				matches = append(matches, symbol)
			}
		}
	}

	slices.SortFunc(matches, func(left, right model.Symbol) int {
		if pathComparison := strings.Compare(filepath.ToSlash(left.Path), filepath.ToSlash(right.Path)); pathComparison != 0 {
			return pathComparison
		}
		if nameComparison := strings.Compare(left.Name, right.Name); nameComparison != 0 {
			return nameComparison
		}
		return left.StartLine - right.StartLine
	})

	return matches
}

// SearchWarnings returns warnings that match the supplied query.
func SearchWarnings(result model.Result, query string) []string {
	normalized := NormalizeQuery(query)
	if normalized == "" {
		return append([]string(nil), result.Warnings...)
	}

	matches := make([]string, 0, len(result.Warnings))
	for _, warning := range result.Warnings {
		if MatchesQuery(warning, normalized) {
			matches = append(matches, warning)
		}
	}

	return matches
}

// FindFile locates a single file by exact normalized path.
func FindFile(result model.Result, path string) (FileMatch, bool) {
	target := NormalizePath(path)
	if target == "" {
		return FileMatch{}, false
	}

	for _, file := range result.FileSummaries {
		if NormalizePath(file.Path) == target {
			return FileMatch{
				Package: workspace.PackageNameForPath(file.Path),
				File:    file,
			}, true
		}
	}

	return FileMatch{}, false
}

// FindPackage locates a package or directory slice by exact normalized name.
func FindPackage(result model.Result, name string) (PackageMatch, bool) {
	target := NormalizePath(name)
	if target == "" {
		return PackageMatch{}, false
	}

	files := make([]model.FileSummary, 0, len(result.FileSummaries))
	for _, file := range result.FileSummaries {
		if NormalizePath(workspace.PackageNameForPath(file.Path)) == target {
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return PackageMatch{}, false
	}

	slices.SortFunc(files, func(left, right model.FileSummary) int {
		return strings.Compare(filepath.ToSlash(left.Path), filepath.ToSlash(right.Path))
	})

	symbolCount := 0
	language := ""
	for _, file := range files {
		if language == "" {
			language = file.Language
		}
		symbolCount += len(file.Symbols)
	}

	dependencies := make([]string, 0, min(6, len(result.Dependencies)))
	for _, dep := range result.Dependencies {
		dependencies = append(dependencies, dep.Path)
		if len(dependencies) == 6 {
			break
		}
	}

	return PackageMatch{
		Package: model.PackageSummary{
			Name:         target,
			Language:     language,
			FileCount:    len(files),
			SymbolCount:  symbolCount,
			Dependencies: dependencies,
		},
		Files: files,
	}, true
}

// FileMatchesQuery checks both file metadata and symbol metadata for a search hit.
func FileMatchesQuery(file model.FileSummary, query string) bool {
	if MatchesQuery(file.Path, query) || MatchesQuery(file.Language, query) {
		return true
	}

	for _, symbol := range file.Symbols {
		if MatchesQuery(symbol.Name, query) || MatchesQuery(symbol.Kind, query) {
			return true
		}
	}

	return false
}

// NormalizeQuery trims and lowercases a free-text query for case-insensitive matching.
func NormalizeQuery(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

// MatchesQuery performs case-insensitive substring matching.
func MatchesQuery(value, query string) bool {
	return strings.Contains(strings.ToLower(value), query)
}

// NormalizePath canonicalizes relative paths and package names for exact lookups.
func NormalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
}
