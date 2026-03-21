package indexer

// IndexRequest describes a single indexing request handled by the CLI.
type IndexRequest struct {
	Indexer                   string         `json:"indexer,omitempty" jsonschema_description:"Optional backend override: auto, go, scip-go, tree-sitter, symbolic-fallback, lsp, or scip-lsp."`
	Root                      string         `json:"root,omitempty" jsonschema_description:"Workspace root to index."`
	Language                  string         `json:"language,omitempty" jsonschema_description:"Optional language override. Use go to force scip-go, otherwise a tree-sitter fallback is used for supported grammars."`
	Paths                     []string       `json:"paths,omitempty" jsonschema_description:"Optional relative files or directories to narrow the indexing scope."`
	SampleLimit               int            `json:"sampleLimit,omitempty" jsonschema_description:"Maximum number of files to symbolically inspect in one request."`
	PageSize                  int            `json:"pageSize,omitempty" jsonschema_description:"Maximum number of result entries to return per page."`
	PageToken                 string         `json:"pageToken,omitempty" jsonschema_description:"Opaque pagination cursor returned by a previous response."`
	IncludeDeps               bool           `json:"includeDeps,omitempty" jsonschema_description:"Include dependency metadata. For Go this returns module dependencies."`
	EmitSCIP                  bool           `json:"emitScip,omitempty" jsonschema_description:"Write a .scip file to disk when enabled."`
	OutputPath                string         `json:"outputPath,omitempty" jsonschema_description:"Optional output file path for the emitted .scip index."`
	SymbolicOnly              bool           `json:"symbolicOnly,omitempty" jsonschema_description:"Return compact symbolic summaries and skip embedding document text."`
	RespectGitIgnore          bool           `json:"respectGitIgnore,omitempty" jsonschema_description:"Reserved for future filtering; currently directory-level defaults are applied."`
	MaxFileBytes              int64          `json:"maxFileBytes,omitempty" jsonschema_description:"Skip files larger than this size in bytes."`
	IncludeHidden             bool           `json:"includeHidden,omitempty" jsonschema_description:"Include hidden files and directories."`
	SummaryDetail             string         `json:"summaryDetail,omitempty" jsonschema_description:"Summary detail level: minimal, normal, or deep."`
	MaxSymbolsPerFile         int            `json:"maxSymbolsPerFile,omitempty" jsonschema_description:"Maximum number of symbols to return per file in the inline response."`
	MaxFiles                  int            `json:"maxFiles,omitempty" jsonschema_description:"Maximum number of ranked files to include in the inline response before trimming."`
	MaxTokensApprox           int            `json:"maxTokensApprox,omitempty" jsonschema_description:"Approximate token budget for inline structured output."`
	IncludeSpans              bool           `json:"includeSpans,omitempty" jsonschema_description:"Include line spans for symbols when true."`
	ResponseMode              string         `json:"responseMode,omitempty" jsonschema_description:"Inline response mode: handles, compact, or detailed."`
	AutoResponseMode          *bool          `json:"autoResponseMode,omitempty" jsonschema_description:"When true, large results can auto-downgrade from compact to handles mode."`
	AutoHandleFileThreshold   int            `json:"autoHandleFileThreshold,omitempty" jsonschema_description:"File threshold for auto-downgrading large responses to handles mode."`
	AutoHandleSymbolThreshold int            `json:"autoHandleSymbolThreshold,omitempty" jsonschema_description:"Symbol threshold for auto-downgrading large responses to handles mode."`
	IncludeDebug              bool           `json:"includeDebug,omitempty" jsonschema_description:"Include debug-only execution details such as command lines."`
	LSPCommand                string         `json:"lspCommand,omitempty" jsonschema_description:"Command used to start a language server when indexer is lsp or scip-lsp."`
	LSPArgs                   []string       `json:"lspArgs,omitempty" jsonschema_description:"Optional arguments passed to the language server process started by lspCommand."`
	LSPEnv                    []string       `json:"lspEnv,omitempty" jsonschema_description:"Optional KEY=VALUE environment entries merged into the language server process environment."`
	LSPInitOptions            map[string]any `json:"lspInitOptions,omitempty" jsonschema_description:"Optional LSP initializationOptions payload passed to the language server during initialize."`
}

// Symbol is a compact symbolic representation extracted from a source file.
type Symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Signature string `json:"signature,omitempty"`
	BodyText  string `json:"bodyText,omitempty"`
}

// FileSummary contains a token-efficient summary of a single indexed file.
type FileSummary struct {
	Path         string   `json:"path"`
	Language     string   `json:"language"`
	Symbols      []Symbol `json:"symbols,omitempty"`
	Bytes        int64    `json:"bytes"`
	Skipped      bool     `json:"skipped,omitempty"`
	SkipReason   string   `json:"skipReason,omitempty"`
	SymbolicSExp string   `json:"symbolicSExp,omitempty"`
}

// ModuleDependency captures dependency metadata returned by the active indexer.
type ModuleDependency struct {
	Path     string `json:"path"`
	Version  string `json:"version,omitempty"`
	Replace  string `json:"replace,omitempty"`
	Main     bool   `json:"main,omitempty"`
	Indirect bool   `json:"indirect,omitempty"`
	Dir      string `json:"dir,omitempty"`
}

// PackageSummary groups indexed files by logical package or directory.
type PackageSummary struct {
	HandleID     string   `json:"handleId,omitempty"`
	Name         string   `json:"name"`
	Language     string   `json:"language"`
	FileCount    int      `json:"fileCount"`
	SymbolCount  int      `json:"symbolCount"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// StringTableEntry stores a deduplicated string used by compact result tables.
type StringTableEntry struct {
	ID    int    `json:"id"`
	Value string `json:"value"`
}

// StringTables stores deduplicated strings by semantic section.
type StringTables struct {
	Paths       []StringTableEntry `json:"paths,omitempty"`
	Languages   []StringTableEntry `json:"languages,omitempty"`
	Packages    []StringTableEntry `json:"packages,omitempty"`
	SymbolNames []StringTableEntry `json:"symbolNames,omitempty"`
	SymbolKinds []StringTableEntry `json:"symbolKinds,omitempty"`
	Misc        []StringTableEntry `json:"misc,omitempty"`
}

// CompactSymbol encodes a symbol by string table IDs and lightweight spans.
type CompactSymbol struct {
	HandleID    string `json:"handleId,omitempty"`
	ID          int    `json:"id"`
	NameID      int    `json:"nameId"`
	KindID      int    `json:"kindId"`
	PathID      int    `json:"pathId"`
	StartLine   int    `json:"startLine,omitempty"`
	EndLine     int    `json:"endLine,omitempty"`
	SpanOmitted bool   `json:"spanOmitted,omitempty"`
}

// CompactFileSummary encodes a file summary using shared string and symbol tables.
type CompactFileSummary struct {
	HandleID     string `json:"handleId"`
	PathID       int    `json:"pathId"`
	LanguageID   int    `json:"languageId"`
	PackageID    int    `json:"packageId,omitempty"`
	Bytes        int64  `json:"bytes"`
	SymbolRefs   []int  `json:"symbolRefs,omitempty"`
	Skipped      bool   `json:"skipped,omitempty"`
	SkipReasonID int    `json:"skipReasonId,omitempty"`
	SExpID       int    `json:"sexpId,omitempty"`
	Score        int    `json:"score,omitempty"`
}

// CompactDependency stores dependency metadata using shared string table IDs.
type CompactDependency struct {
	PathID    int  `json:"pathId"`
	VersionID int  `json:"versionId,omitempty"`
	ReplaceID int  `json:"replaceId,omitempty"`
	DirID     int  `json:"dirId,omitempty"`
	Main      bool `json:"main,omitempty"`
	Indirect  bool `json:"indirect,omitempty"`
}

// WarningNotice stores a warning code and one canonical message.
type WarningNotice struct {
	Code      string `json:"code"`
	MessageID int    `json:"messageId,omitempty"`
	Count     int    `json:"count,omitempty"`
}

// DebugInfo contains opt-in execution details.
type DebugInfo struct {
	CommandLine []string `json:"commandLine,omitempty"`
}

// Relationship is a lightweight edge between compact entities.
type Relationship struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// ResultBudget reports how aggressively the inline response was trimmed.
type ResultBudget struct {
	Applied           bool     `json:"applied"`
	SummaryDetail     string   `json:"summaryDetail"`
	MaxFiles          int      `json:"maxFiles,omitempty"`
	MaxSymbolsPerFile int      `json:"maxSymbolsPerFile,omitempty"`
	MaxTokensApprox   int      `json:"maxTokensApprox,omitempty"`
	ResponseMode      string   `json:"responseMode,omitempty"`
	Truncated         bool     `json:"truncated,omitempty"`
	OmittedFields     []string `json:"omittedFields,omitempty"`
}

// Result is the structured response returned after indexing.
//
// Some fields remain optional or lightly used because the same result type is
// shared by one-shot index output and the newer query-oriented CLI commands.
type Result struct {
	Indexer        string               `json:"indexer"`
	Root           string               `json:"root"`
	Language       string               `json:"language"`
	FilesScanned   int                  `json:"filesScanned"`
	FilesIndexed   int                  `json:"filesIndexed"`
	FilesSkipped   int                  `json:"filesSkipped"`
	Sampled        bool                 `json:"sampled"`
	SCIPPath       string               `json:"scipPath,omitempty"`
	Dependencies   []ModuleDependency   `json:"dependencies,omitempty"`
	FileSummaries  []FileSummary        `json:"fileSummaries,omitempty"`
	Packages       []PackageSummary     `json:"packages,omitempty"`
	StringTables   StringTables         `json:"stringTables"`
	CompactDeps    []CompactDependency  `json:"compactDependencies,omitempty"`
	CompactSymbols []CompactSymbol      `json:"compactSymbols,omitempty"`
	CompactFiles   []CompactFileSummary `json:"compactFiles,omitempty"`
	Relationships  []Relationship       `json:"relationships,omitempty"`
	ResultHandle   string               `json:"resultHandle,omitempty"`
	Warnings       []string             `json:"warnings,omitempty"`
	WorkspaceDirty bool                 `json:"workspaceDirty,omitempty"`
	FullWarnings   []string             `json:"-"`
	CommandLine    []string             `json:"-"`
	WarningNotices []WarningNotice      `json:"warningNotices,omitempty"`
	Debug          *DebugInfo           `json:"debug,omitempty"`
	// Legacy capability flags are preserved for backward compatibility with older
	// result consumers even though the supported product surface is now the CLI.
	UsedClientRoots  bool         `json:"usedClientRoots,omitempty"`
	UsedElicitation  bool         `json:"usedElicitation,omitempty"`
	UsedClientSample bool         `json:"usedClientSample,omitempty"`
	ClientSummary    string       `json:"clientSummary,omitempty"`
	PageSize         int          `json:"pageSize,omitempty"`
	NextPageToken    string       `json:"nextPageToken,omitempty"`
	TotalFiles       int          `json:"totalFiles,omitempty"`
	ReturnedFiles    int          `json:"returnedFiles,omitempty"`
	TotalDeps        int          `json:"totalDependencies,omitempty"`
	ReturnedDeps     int          `json:"returnedDependencies,omitempty"`
	Budget           ResultBudget `json:"budget"`
	ToolHints        []string     `json:"toolHints,omitempty"`
}
