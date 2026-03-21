package indexer

import "strings"

func effectiveResponseMode(req IndexRequest, result Result) (string, bool) {
	mode := req.ResponseMode
	if mode == "" {
		mode = "compact"
	}
	if mode == "detailed" || !autoResponseModeEnabled(req) {
		return mode, false
	}
	fileThreshold := req.AutoHandleFileThreshold
	if fileThreshold <= 0 {
		fileThreshold = DefaultAutoHandleFileThreshold
	}
	symbolThreshold := req.AutoHandleSymbolThreshold
	if symbolThreshold <= 0 {
		symbolThreshold = DefaultAutoHandleSymbolThreshold
	}
	if result.FilesIndexed >= fileThreshold || len(result.CompactSymbols) >= symbolThreshold {
		if mode != "handles" {
			return "handles", true
		}
	}
	return mode, false
}

func autoResponseModeEnabled(req IndexRequest) bool {
	return req.AutoResponseMode == nil || *req.AutoResponseMode
}

func applyBudgetFallback(result Result, base Result, req IndexRequest, mode string, packageFirst bool) Result {
	if estimateResultTokens(result) <= req.MaxTokensApprox {
		return result
	}

	fallbackModes := []string{}
	switch mode {
	case "detailed":
		fallbackModes = []string{"compact", "handles"}
	case "compact":
		fallbackModes = []string{"handles"}
	}

	for _, fallback := range fallbackModes {
		candidate := applyResponseMode(base, fallback, packageFirst)
		candidate.Budget = result.Budget
		candidate.Budget.Truncated = true
		candidate.Budget.ResponseMode = fallback
		candidate.Budget.OmittedFields = uniqueStrings(append(candidate.Budget.OmittedFields, "budgetMode:"+fallback))
		applyModeOmissions(&candidate.Budget, fallback, packageFirst)
		if estimateResultTokens(candidate) <= req.MaxTokensApprox {
			return candidate
		}
		result = candidate
	}

	if len(result.Relationships) > 0 {
		result.Relationships = dropSymbolRelationships(result.Relationships)
		result.Budget.Truncated = true
		result.Budget.OmittedFields = uniqueStrings(append(result.Budget.OmittedFields, "symbolRelationships"))
		if estimateResultTokens(result) <= req.MaxTokensApprox {
			return result
		}
		result.Relationships = nil
		result.Budget.OmittedFields = uniqueStrings(append(result.Budget.OmittedFields, "relationships"))
	}
	if len(result.Warnings) > 0 {
		result.Warnings = nil
		result.Budget.OmittedFields = uniqueStrings(append(result.Budget.OmittedFields, "inlineWarnings"))
		if estimateResultTokens(result) <= req.MaxTokensApprox {
			return result
		}
	}
	if len(result.CompactSymbols) > 0 {
		result.CompactSymbols = nil
		for i := range result.CompactFiles {
			result.CompactFiles[i].SymbolRefs = nil
		}
		result.Budget.OmittedFields = uniqueStrings(append(result.Budget.OmittedFields, "compactSymbols"))
	}

	return result
}

func applyResponseMode(base Result, mode string, packageFirst bool) Result {
	result := cloneResult(base)
	switch mode {
	case "handles":
		for i := range result.CompactFiles {
			result.CompactFiles[i].SymbolRefs = nil
			result.CompactFiles[i].SExpID = 0
		}
		result.FileSummaries = nil
		result.CompactSymbols = nil
		result.Relationships = nil
	case "compact":
		result.FileSummaries = nil
	}
	if packageFirst && mode != "detailed" {
		result.FileSummaries = nil
		if mode == "compact" {
			result.CompactSymbols = nil
		}
	}
	return result
}

func applyModeOmissions(budget *ResultBudget, mode string, packageFirst bool) {
	switch mode {
	case "handles":
		budget.OmittedFields = uniqueStrings(append(budget.OmittedFields, "inlineFileSummaries", "compactSymbols", "relationships"))
	case "compact":
		budget.OmittedFields = uniqueStrings(append(budget.OmittedFields, "inlineFileSummaries"))
	}
	if packageFirst && mode != "detailed" {
		budget.OmittedFields = uniqueStrings(append(budget.OmittedFields, "compactSymbols"))
	}
}

func cloneResult(base Result) Result {
	result := base
	result.FileSummaries = append([]FileSummary(nil), base.FileSummaries...)
	result.Packages = append([]PackageSummary(nil), base.Packages...)
	result.CompactDeps = append([]CompactDependency(nil), base.CompactDeps...)
	result.CompactSymbols = append([]CompactSymbol(nil), base.CompactSymbols...)
	result.CompactFiles = make([]CompactFileSummary, len(base.CompactFiles))
	for i, file := range base.CompactFiles {
		copyFile := file
		copyFile.SymbolRefs = append([]int(nil), file.SymbolRefs...)
		result.CompactFiles[i] = copyFile
	}
	result.Relationships = append([]Relationship(nil), base.Relationships...)
	result.Warnings = append([]string(nil), base.Warnings...)
	result.WarningNotices = append([]WarningNotice(nil), base.WarningNotices...)
	result.FullWarnings = append([]string(nil), base.FullWarnings...)
	if base.Debug != nil {
		debug := *base.Debug
		debug.CommandLine = append([]string(nil), base.Debug.CommandLine...)
		result.Debug = &debug
	}
	result.StringTables = StringTables{
		Paths:       append([]StringTableEntry(nil), base.StringTables.Paths...),
		Languages:   append([]StringTableEntry(nil), base.StringTables.Languages...),
		Packages:    append([]StringTableEntry(nil), base.StringTables.Packages...),
		SymbolNames: append([]StringTableEntry(nil), base.StringTables.SymbolNames...),
		SymbolKinds: append([]StringTableEntry(nil), base.StringTables.SymbolKinds...),
		Misc:        append([]StringTableEntry(nil), base.StringTables.Misc...),
	}
	return result
}

func dropSymbolRelationships(relationships []Relationship) []Relationship {
	filtered := make([]Relationship, 0, len(relationships))
	for _, relationship := range relationships {
		if strings.HasPrefix(relationship.To, "symbol:") {
			continue
		}
		filtered = append(filtered, relationship)
	}
	return filtered
}
