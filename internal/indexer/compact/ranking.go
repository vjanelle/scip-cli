package compact

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/vjanelle/scip-cli/internal/indexer/model"
	"github.com/vjanelle/scip-cli/internal/indexer/workspace"
)

type rankedFileSummary struct {
	summary model.FileSummary
	score   int
}

func rankFileSummaries(files []model.FileSummary) []rankedFileSummary {
	ranked := make([]rankedFileSummary, 0, len(files))
	for _, file := range files {
		score := len(file.Symbols)*20 + scorePath(file.Path) + len(file.SymbolicSExp)/24
		if filepath.Base(file.Path) == "main.go" || strings.HasSuffix(file.Path, "/main.go") {
			score += 60
		}
		ranked = append(ranked, rankedFileSummary{summary: file, score: score})
	}

	slices.SortFunc(ranked, func(a, b rankedFileSummary) int {
		if a.score == b.score {
			return strings.Compare(a.summary.Path, b.summary.Path)
		}
		return b.score - a.score
	})
	return ranked
}

func scorePath(path string) int {
	score := 0
	if !strings.Contains(path, string(filepath.Separator)) && !strings.Contains(path, "/") {
		score += 20
	}
	if strings.Contains(strings.ToLower(path), "cmd") || strings.Contains(strings.ToLower(path), "api") {
		score += 12
	}
	if strings.Contains(strings.ToLower(path), "test") {
		score -= 10
	}
	return score
}

func trimFileSummaryForBudget(file model.FileSummary, req model.IndexRequest) model.FileSummary {
	trimmed := file
	symbols := append([]model.Symbol(nil), file.Symbols...)
	if len(symbols) > req.MaxSymbolsPerFile {
		symbols = symbols[:req.MaxSymbolsPerFile]
	}

	switch req.SummaryDetail {
	case "minimal":
		for i := range symbols {
			symbols[i].Name = ""
			symbols[i].StartLine = 0
			symbols[i].EndLine = 0
		}
		trimmed.SymbolicSExp = ""
	case "normal":
		if !req.IncludeSpans {
			for i := range symbols {
				symbols[i].StartLine = 0
				symbols[i].EndLine = 0
			}
		}
		if len(trimmed.SymbolicSExp) > 160 {
			trimmed.SymbolicSExp = trimmed.SymbolicSExp[:160] + "..."
		}
	case "deep":
		if len(trimmed.SymbolicSExp) > 320 {
			trimmed.SymbolicSExp = trimmed.SymbolicSExp[:320] + "..."
		}
	}

	trimmed.Symbols = symbols
	return trimmed
}

func buildPackageSummaries(result model.Result, files []model.FileSummary) []model.PackageSummary {
	type bucket struct {
		lang    string
		files   int
		symbols int
	}
	pkgs := map[string]*bucket{}
	for _, file := range files {
		name := workspace.PackageNameForPath(file.Path)
		entry := pkgs[name]
		if entry == nil {
			entry = &bucket{lang: file.Language}
			pkgs[name] = entry
		}
		entry.files++
		entry.symbols += len(file.Symbols)
	}

	names := make([]string, 0, len(pkgs))
	for name := range pkgs {
		names = append(names, name)
	}
	slices.Sort(names)

	deps := make([]string, 0, min(6, len(result.Dependencies)))
	for _, dep := range result.Dependencies {
		deps = append(deps, dep.Path)
		if len(deps) == 6 {
			break
		}
	}

	out := make([]model.PackageSummary, 0, len(names))
	for _, name := range names {
		entry := pkgs[name]
		out = append(out, model.PackageSummary{
			Name:         name,
			Language:     entry.lang,
			FileCount:    entry.files,
			SymbolCount:  entry.symbols,
			Dependencies: deps,
		})
	}
	return out
}
