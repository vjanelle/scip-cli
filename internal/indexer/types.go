package indexer

import "github.com/vjanelle/scip-cli/internal/indexer/model"

// These aliases keep the public indexer package stable while shared models live in a focused subpackage.
type (
	IndexRequest       = model.IndexRequest
	Symbol             = model.Symbol
	FileSummary        = model.FileSummary
	ModuleDependency   = model.ModuleDependency
	PackageSummary     = model.PackageSummary
	StringTableEntry   = model.StringTableEntry
	StringTables       = model.StringTables
	CompactSymbol      = model.CompactSymbol
	CompactFileSummary = model.CompactFileSummary
	CompactDependency  = model.CompactDependency
	WarningNotice      = model.WarningNotice
	DebugInfo          = model.DebugInfo
	Relationship       = model.Relationship
	ResultBudget       = model.ResultBudget
	Result             = model.Result
)
