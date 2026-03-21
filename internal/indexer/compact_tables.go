package indexer

import (
	"fmt"
	"path/filepath"
	"strings"
)

func buildCompactStructures(files []FileSummary, packages []PackageSummary, req IndexRequest) (*stringTablesBuilder, []CompactSymbol, []CompactFileSummary, []Relationship) {
	table := newStringTablesBuilder()
	packageIDs := map[string]int{}
	for _, pkg := range packages {
		packageIDs[pkg.Name] = table.add("packages", pkg.Name)
	}

	compactSymbols := make([]CompactSymbol, 0, len(files)*req.MaxSymbolsPerFile)
	compactFiles := make([]CompactFileSummary, 0, len(files))
	relationships := make([]Relationship, 0, len(files)*2)
	symbolID := 1

	for index, file := range files {
		pathKey := normalizePathKey(file.Path)
		pathID := table.add("paths", pathKey)
		languageID := table.add("languages", file.Language)
		packageName := PackageNameForPath(pathKey)
		fileHandle := fmt.Sprintf("file:%s", pathKey)
		compacted := CompactFileSummary{
			HandleID:   fileHandle,
			PathID:     pathID,
			LanguageID: languageID,
			PackageID:  packageIDs[packageName],
			Bytes:      file.Bytes,
			Score:      len(file.Symbols)*20 + index,
		}
		if file.SkipReason != "" {
			compacted.SkipReasonID = table.add("misc", file.SkipReason)
		}
		if file.SymbolicSExp != "" {
			compacted.SExpID = table.add("misc", file.SymbolicSExp)
		}

		for _, symbol := range file.Symbols {
			entry := CompactSymbol{
				ID:          symbolID,
				NameID:      table.add("symbolNames", symbol.Name),
				KindID:      table.add("symbolKinds", symbol.Kind),
				PathID:      pathID,
				SpanOmitted: !req.IncludeSpans || symbol.StartLine == 0,
			}
			if req.IncludeSpans {
				entry.StartLine = symbol.StartLine
				entry.EndLine = symbol.EndLine
			}
			compactSymbols = append(compactSymbols, entry)
			compacted.SymbolRefs = append(compacted.SymbolRefs, symbolID)
			relationships = append(relationships, Relationship{
				From: fileHandle,
				To:   fmt.Sprintf("symbol:%d", symbolID),
				Kind: "contains",
			})
			symbolID++
		}

		relationships = append(relationships,
			Relationship{From: "package:" + packageName, To: fileHandle, Kind: "contains"},
			Relationship{From: "root", To: fileHandle, Kind: "indexes"},
		)
		compactFiles = append(compactFiles, compacted)
	}

	return table, compactSymbols, compactFiles, relationships
}

// normalizePathKey canonicalizes paths before string-table insertion.
func normalizePathKey(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.ToSlash(filepath.Clean(path))
	path = strings.TrimPrefix(path, "./")
	if path == "." {
		return "root"
	}
	return path
}

func estimateResultTokens(result Result) int {
	estimate := len(result.CompactFiles)*12 + len(result.CompactSymbols)*8 + len(result.Relationships)*6 + len(result.CompactDeps)*6 + len(result.WarningNotices)*4
	for _, section := range [][]StringTableEntry{
		result.StringTables.Paths,
		result.StringTables.Languages,
		result.StringTables.Packages,
		result.StringTables.SymbolNames,
		result.StringTables.SymbolKinds,
		result.StringTables.Misc,
	} {
		for _, entry := range section {
			estimate += len(entry.Value) / 4
		}
	}
	return estimate
}

type stringTablesBuilder struct {
	ids    map[string]map[string]int
	values map[string][]StringTableEntry
}

func newStringTablesBuilder() *stringTablesBuilder {
	return &stringTablesBuilder{
		ids: map[string]map[string]int{},
		values: map[string][]StringTableEntry{
			"paths":       {},
			"languages":   {},
			"packages":    {},
			"symbolNames": {},
			"symbolKinds": {},
			"misc":        {},
		},
	}
}

func (b *stringTablesBuilder) add(section, value string) int {
	if value == "" {
		return 0
	}
	if b.ids[section] == nil {
		b.ids[section] = map[string]int{}
	}
	if id, ok := b.ids[section][value]; ok {
		return id
	}
	id := len(b.values[section]) + 1
	b.ids[section][value] = id
	b.values[section] = append(b.values[section], StringTableEntry{ID: id, Value: value})
	return id
}

func (b *stringTablesBuilder) entries() StringTables {
	return StringTables{
		Paths:       append([]StringTableEntry(nil), b.values["paths"]...),
		Languages:   append([]StringTableEntry(nil), b.values["languages"]...),
		Packages:    append([]StringTableEntry(nil), b.values["packages"]...),
		SymbolNames: append([]StringTableEntry(nil), b.values["symbolNames"]...),
		SymbolKinds: append([]StringTableEntry(nil), b.values["symbolKinds"]...),
		Misc:        append([]StringTableEntry(nil), b.values["misc"]...),
	}
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
