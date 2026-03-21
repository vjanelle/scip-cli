package indexer

import "github.com/vjanelle/scip-cli/internal/indexer/queries"

// These aliases keep query result types stable while the search logic lives in a focused helper package.
type (
	FileMatch    = queries.FileMatch
	PackageMatch = queries.PackageMatch
)

// SearchFiles matches files by path and symbol name using a case-insensitive contains query.
func SearchFiles(result Result, query string) []FileMatch {
	return queries.SearchFiles(result, query)
}

// SearchSymbols returns matching symbols across all indexed files.
func SearchSymbols(result Result, query string) []Symbol {
	return queries.SearchSymbols(result, query)
}

// SearchWarnings returns warnings that match the supplied query.
func SearchWarnings(result Result, query string) []string {
	return queries.SearchWarnings(result, query)
}

// FindFile locates a single file by exact normalized path.
func FindFile(result Result, path string) (FileMatch, bool) {
	return queries.FindFile(result, path)
}

// FindPackage locates a package or directory slice by exact normalized name.
func FindPackage(result Result, name string) (PackageMatch, bool) {
	return queries.FindPackage(result, name)
}

// fileMatchesQuery remains here as a package-local compatibility shim for tests.
func fileMatchesQuery(file FileSummary, query string) bool {
	return queries.FileMatchesQuery(file, query)
}

// normalizeQuery remains here as a package-local compatibility shim for tests.
func normalizeQuery(query string) string {
	return queries.NormalizeQuery(query)
}

// matchesQuery remains here as a package-local compatibility shim for tests.
func matchesQuery(value, query string) bool {
	return queries.MatchesQuery(value, query)
}

// normalizePath remains here as a package-local compatibility shim for tests.
func normalizePath(path string) string {
	return queries.NormalizePath(path)
}
