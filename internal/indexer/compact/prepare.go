package compact

import "github.com/vjanelle/scip-cli/internal/indexer/model"

// PrepareResult reduces inline payload size by ranking files, trimming detail,
// and building compact dictionary-backed structures.
func PrepareResult(req model.IndexRequest, result model.Result) model.Result {
	ranked := rankFileSummaries(result.FileSummaries)
	truncated := false
	if len(ranked) > req.MaxFiles {
		ranked = ranked[:req.MaxFiles]
		truncated = true
	}

	inlineFiles := make([]model.FileSummary, 0, len(ranked))
	omitted := make([]string, 0, 8)
	for _, rankedFile := range ranked {
		file := trimFileSummaryForBudget(rankedFile.summary, req)
		if len(file.Symbols) < len(rankedFile.summary.Symbols) || file.SymbolicSExp == "" {
			truncated = true
		}
		inlineFiles = append(inlineFiles, file)
	}

	if req.SummaryDetail == "minimal" {
		omitted = append(omitted, "symbolNames", "lineSpans")
	} else if !req.IncludeSpans {
		omitted = append(omitted, "lineSpans")
	}
	if len(result.FileSummaries) > len(inlineFiles) {
		omitted = append(omitted, "additionalFiles")
	}
	packageFirst := req.ResponseMode != "detailed" && len(result.FileSummaries) > req.MaxFiles*2
	if packageFirst {
		omitted = append(omitted, "packageFirst")
	}

	packages := buildPackageSummaries(result, result.FileSummaries)
	table, compactSymbols, compactFiles, relationships := buildCompactStructures(inlineFiles, packages, req)

	fullWarnings := append([]string(nil), result.Warnings...)
	warnings, notices := compactWarnings(fullWarnings, table)

	base := result
	base.FileSummaries = inlineFiles
	base.Packages = packages
	base.StringTables = table.entries()
	base.CompactDeps = buildCompactDependencies(result.Dependencies, table)
	base.CompactSymbols = compactSymbols
	base.CompactFiles = compactFiles
	base.Relationships = relationships
	base.FullWarnings = fullWarnings
	base.Warnings = warnings
	base.WarningNotices = notices
	if !req.IncludeDebug {
		base.Debug = nil
	} else if base.Debug == nil && len(base.CommandLine) > 0 {
		base.Debug = &model.DebugInfo{CommandLine: append([]string(nil), base.CommandLine...)}
	}
	base.CommandLine = nil

	mode, autoDowngraded := effectiveResponseMode(req, base)
	shaped := applyResponseMode(base, mode, packageFirst)
	shaped.Budget = model.ResultBudget{
		Applied:           true,
		SummaryDetail:     req.SummaryDetail,
		MaxFiles:          req.MaxFiles,
		MaxSymbolsPerFile: req.MaxSymbolsPerFile,
		MaxTokensApprox:   req.MaxTokensApprox,
		ResponseMode:      mode,
		Truncated:         truncated || autoDowngraded,
		OmittedFields:     uniqueStrings(omitted),
	}
	applyModeOmissions(&shaped.Budget, mode, packageFirst)
	if autoDowngraded {
		shaped.Budget.OmittedFields = uniqueStrings(append(shaped.Budget.OmittedFields, "autoHandles"))
	}

	return applyBudgetFallback(shaped, base, req, mode, packageFirst)
}
