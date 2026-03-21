package indexer

import (
	"path/filepath"
	"slices"
	"strings"
)

// FileMatch is a file-level query hit enriched with its logical package label.
type FileMatch struct {
	Package string      `json:"package"`
	File    FileSummary `json:"file"`
}

// PackageMatch is a package-level query hit with the files that belong to it.
type PackageMatch struct {
	Package PackageSummary `json:"package"`
	Files   []FileSummary  `json:"files"`
}

// SearchFiles matches files by path and symbol name using a case-insensitive contains query.
func SearchFiles(result Result, query string) []FileMatch {
	normalized := normalizeQuery(query)
	if normalized == "" {
		return nil
	}

	matches := make([]FileMatch, 0, len(result.FileSummaries))
	for _, file := range result.FileSummaries {
		if fileMatchesQuery(file, normalized) {
			matches = append(matches, FileMatch{
				Package: PackageNameForPath(file.Path),
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
func SearchSymbols(result Result, query string) []Symbol {
	normalized := normalizeQuery(query)
	if normalized == "" {
		return nil
	}

	matches := make([]Symbol, 0, 16)
	for _, file := range result.FileSummaries {
		for _, symbol := range file.Symbols {
			if matchesQuery(symbol.Name, normalized) || matchesQuery(symbol.Kind, normalized) || matchesQuery(symbol.Path, normalized) {
				matches = append(matches, symbol)
			}
		}
	}

	slices.SortFunc(matches, func(left, right Symbol) int {
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
func SearchWarnings(result Result, query string) []string {
	normalized := normalizeQuery(query)
	if normalized == "" {
		return append([]string(nil), result.Warnings...)
	}

	matches := make([]string, 0, len(result.Warnings))
	for _, warning := range result.Warnings {
		if matchesQuery(warning, normalized) {
			matches = append(matches, warning)
		}
	}

	return matches
}

// FindFile locates a single file by exact normalized path.
func FindFile(result Result, path string) (FileMatch, bool) {
	target := normalizePath(path)
	if target == "" {
		return FileMatch{}, false
	}

	for _, file := range result.FileSummaries {
		if normalizePath(file.Path) == target {
			return FileMatch{
				Package: PackageNameForPath(file.Path),
				File:    file,
			}, true
		}
	}

	return FileMatch{}, false
}

// FindPackage locates a package or directory slice by exact normalized name.
func FindPackage(result Result, name string) (PackageMatch, bool) {
	target := normalizePath(name)
	if target == "" {
		return PackageMatch{}, false
	}

	files := make([]FileSummary, 0, len(result.FileSummaries))
	for _, file := range result.FileSummaries {
		if normalizePath(PackageNameForPath(file.Path)) == target {
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return PackageMatch{}, false
	}

	slices.SortFunc(files, func(left, right FileSummary) int {
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
		Package: PackageSummary{
			Name:         target,
			Language:     language,
			FileCount:    len(files),
			SymbolCount:  symbolCount,
			Dependencies: dependencies,
		},
		Files: files,
	}, true
}

// fileMatchesQuery checks both file metadata and symbol metadata for a search hit.
func fileMatchesQuery(file FileSummary, query string) bool {
	if matchesQuery(file.Path, query) || matchesQuery(file.Language, query) {
		return true
	}

	for _, symbol := range file.Symbols {
		if matchesQuery(symbol.Name, query) || matchesQuery(symbol.Kind, query) {
			return true
		}
	}

	return false
}

// normalizeQuery trims and lowercases a free-text query for case-insensitive matching.
func normalizeQuery(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

// matchesQuery performs case-insensitive substring matching.
func matchesQuery(value, query string) bool {
	return strings.Contains(strings.ToLower(value), query)
}

// normalizePath canonicalizes relative paths and package names for exact lookups.
func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
}
