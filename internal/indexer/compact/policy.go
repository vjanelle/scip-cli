package compact

import (
	"strings"

	"github.com/vjanelle/scip-cli/internal/indexer/model"
	"github.com/vjanelle/scip-cli/internal/indexer/workspace"
)

func effectiveResponseMode(req model.IndexRequest, result model.Result) (string, bool) {
	mode := req.ResponseMode
	if mode == "" {
		mode = "compact"
	}
	if mode == "detailed" || !autoResponseModeEnabled(req) {
		return mode, false
	}
	fileThreshold := req.AutoHandleFileThreshold
	if fileThreshold <= 0 {
		fileThreshold = workspace.DefaultAutoHandleFileThreshold
	}
	symbolThreshold := req.AutoHandleSymbolThreshold
	if symbolThreshold <= 0 {
		symbolThreshold = workspace.DefaultAutoHandleSymbolThreshold
	}
	if result.FilesIndexed >= fileThreshold || len(result.CompactSymbols) >= symbolThreshold {
		if mode != "handles" {
			return "handles", true
		}
	}
	return mode, false
}

func autoResponseModeEnabled(req model.IndexRequest) bool {
	return req.AutoResponseMode == nil || *req.AutoResponseMode
}

func applyBudgetFallback(result model.Result, base model.Result, req model.IndexRequest, mode string, packageFirst bool) model.Result {
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

func applyResponseMode(base model.Result, mode string, packageFirst bool) model.Result {
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

func applyModeOmissions(budget *model.ResultBudget, mode string, packageFirst bool) {
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

func cloneResult(base model.Result) model.Result {
	result := base
	result.FileSummaries = append([]model.FileSummary(nil), base.FileSummaries...)
	result.Packages = append([]model.PackageSummary(nil), base.Packages...)
	result.CompactDeps = append([]model.CompactDependency(nil), base.CompactDeps...)
	result.CompactSymbols = append([]model.CompactSymbol(nil), base.CompactSymbols...)
	result.CompactFiles = make([]model.CompactFileSummary, len(base.CompactFiles))
	for i, file := range base.CompactFiles {
		copyFile := file
		copyFile.SymbolRefs = append([]int(nil), file.SymbolRefs...)
		result.CompactFiles[i] = copyFile
	}
	result.Relationships = append([]model.Relationship(nil), base.Relationships...)
	result.Warnings = append([]string(nil), base.Warnings...)
	result.WarningNotices = append([]model.WarningNotice(nil), base.WarningNotices...)
	result.FullWarnings = append([]string(nil), base.FullWarnings...)
	if base.Debug != nil {
		debug := *base.Debug
		debug.CommandLine = append([]string(nil), base.Debug.CommandLine...)
		result.Debug = &debug
	}
	result.StringTables = model.StringTables{
		Paths:       append([]model.StringTableEntry(nil), base.StringTables.Paths...),
		Languages:   append([]model.StringTableEntry(nil), base.StringTables.Languages...),
		Packages:    append([]model.StringTableEntry(nil), base.StringTables.Packages...),
		SymbolNames: append([]model.StringTableEntry(nil), base.StringTables.SymbolNames...),
		SymbolKinds: append([]model.StringTableEntry(nil), base.StringTables.SymbolKinds...),
		Misc:        append([]model.StringTableEntry(nil), base.StringTables.Misc...),
	}
	return result
}

func dropSymbolRelationships(relationships []model.Relationship) []model.Relationship {
	filtered := make([]model.Relationship, 0, len(relationships))
	for _, relationship := range relationships {
		if strings.HasPrefix(relationship.To, "symbol:") {
			continue
		}
		filtered = append(filtered, relationship)
	}
	return filtered
}
