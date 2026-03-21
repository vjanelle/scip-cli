package main

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/vjanelle/scip-cli/internal/indexer"
)

// normalizeOutputFormat accepts both `markdown` and the `md` shorthand.
func normalizeOutputFormat(raw string) (outputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(outputFormatJSON):
		return outputFormatJSON, nil
	case "md", string(outputFormatMarkdown):
		return outputFormatMarkdown, nil
	case string(outputFormatText):
		return outputFormatText, nil
	default:
		return "", fmt.Errorf("unsupported output format %q", raw)
	}
}

func renderIndexResult(stdout io.Writer, options indexOptions, result indexer.Result) error {
	format, err := normalizeOutputFormat(options.format)
	if err != nil {
		return err
	}

	switch format {
	case outputFormatJSON:
		return writeJSON(stdout, result, options.pretty)
	case outputFormatMarkdown:
		return writeMarkdownIndexResult(stdout, result)
	case outputFormatText:
		return writeTextIndexResult(stdout, result)
	default:
		return fmt.Errorf("unsupported output format %q", options.format)
	}
}

// writeJSON emits a structured value for automation-friendly consumers.
func writeJSON(stdout io.Writer, value any, pretty bool) error {
	encoder := json.NewEncoder(stdout)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(value)
}

func writeMarkdownIndexResult(stdout io.Writer, result indexer.Result) error {
	return renderMarkdownDocument(stdout, buildIndexMarkdownDocument(result))
}

// writeTextIndexResult keeps the text serializer phase-oriented while the
// renderWriter helper absorbs repetitive write error handling.
func writeTextIndexResult(stdout io.Writer, result indexer.Result) error {
	return withRenderWriter(stdout, func(writer renderWriter) {
		writeTextIndexSummary(writer, result)
		writeTextIndexFiles(writer, result)
		writeTextIndexDependencies(writer, result)
		writeTextIndexWarnings(writer, result)
	})
}

func writeTextIndexSummary(writer renderWriter, result indexer.Result) {
	writer.Fprintf("Indexer: %s\nRoot: %s\n", result.Indexer, result.Root)
	if result.Language != "" {
		writer.Fprintf("Language: %s\n", result.Language)
	}
	writer.Fprintf("Files: indexed %d of %d scanned", result.FilesIndexed, result.FilesScanned)
	if result.FilesSkipped > 0 {
		writer.Fprintf(" (%d skipped)", result.FilesSkipped)
	}
	writer.Fprintln()
}

func writeTextIndexFiles(writer renderWriter, result indexer.Result) {
	if len(result.FileSummaries) > 0 {
		writer.Fprintln("\nTop files:")
		for _, file := range result.FileSummaries {
			writer.Fprintf("- %s [%s] %d bytes\n", file.Path, file.Language, file.Bytes)
			if names := symbolNames(file.Symbols); len(names) > 0 {
				writer.Fprintf("  symbols: %s\n", strings.Join(names, ", "))
			}
		}
		return
	}
	if len(result.CompactFiles) > 0 {
		writeCompactTextFiles(writer, result)
	}
}

func writeTextIndexDependencies(writer renderWriter, result indexer.Result) {
	if len(result.Dependencies) == 0 {
		return
	}
	writer.Fprintf("\nDependencies: %d\n", len(result.Dependencies))
}

func writeTextIndexWarnings(writer renderWriter, result indexer.Result) {
	if len(result.Warnings) == 0 {
		return
	}
	writer.Fprintln("\nWarnings:")
	for _, warning := range result.Warnings {
		writer.Fprintf("- %s\n", warning)
	}
}

func writeMarkdownSearchFiles(stdout io.Writer, output searchFilesOutput) error {
	return renderMarkdownDocument(stdout, buildSearchFilesMarkdownDocument(output))
}

func writeTextSearchFiles(stdout io.Writer, output searchFilesOutput) error {
	return withRenderWriter(stdout, func(writer renderWriter) {
		writer.Fprintf("Query: %s\nMatches: %d\n", output.Query, output.TotalMatches)
		for _, match := range output.Files {
			writer.Fprintf("- %s [%s]\n", match.File.Path, match.Package)
		}
	})
}

func writeMarkdownSearchSymbols(stdout io.Writer, output searchSymbolsOutput) error {
	return renderMarkdownDocument(stdout, buildSearchSymbolsMarkdownDocument(output))
}

func writeTextSearchSymbols(stdout io.Writer, output searchSymbolsOutput) error {
	return withRenderWriter(stdout, func(writer renderWriter) {
		writer.Fprintf("Query: %s\nMatches: %d\n", output.Query, output.TotalMatches)
		for _, symbol := range output.Symbols {
			writer.Fprintf("- %s [%s] %s\n", symbol.Name, symbol.Kind, symbol.Path)
		}
	})
}

func writeMarkdownSearchWarnings(stdout io.Writer, output searchWarningsOutput) error {
	return renderMarkdownDocument(stdout, buildSearchWarningsMarkdownDocument(output))
}

func writeTextSearchWarnings(stdout io.Writer, output searchWarningsOutput) error {
	return withRenderWriter(stdout, func(writer renderWriter) {
		writer.Fprintf("Query: %s\nMatches: %d\n", output.Query, output.TotalMatches)
		for _, warning := range output.Warnings {
			writer.Fprintf("- %s\n", warning)
		}
	})
}

func writeMarkdownShowFile(stdout io.Writer, output showFileOutput) error {
	return renderMarkdownDocument(stdout, buildShowFileMarkdownDocument(output))
}

func writeTextShowFile(stdout io.Writer, output showFileOutput) error {
	return withRenderWriter(stdout, func(writer renderWriter) {
		writer.Fprintf("Path: %s\nPackage: %s\n", output.Match.File.Path, output.Match.Package)
		for _, symbol := range output.Match.File.Symbols {
			writer.Fprintf("- %s [%s] %d-%d\n", symbol.Name, symbol.Kind, symbol.StartLine, symbol.EndLine)
		}
	})
}

func writeMarkdownShowPackage(stdout io.Writer, output showPackageOutput) error {
	return renderMarkdownDocument(stdout, buildShowPackageMarkdownDocument(output))
}

func writeTextShowPackage(stdout io.Writer, output showPackageOutput) error {
	return withRenderWriter(stdout, func(writer renderWriter) {
		writer.Fprintf("Package: %s\nFiles: %d\nSymbols: %d\n", output.Match.Package.Name, output.Match.Package.FileCount, output.Match.Package.SymbolCount)
		for _, file := range output.Match.Files {
			writer.Fprintf("- %s\n", file.Path)
		}
	})
}

func writeCompactTextFiles(writer renderWriter, result indexer.Result) {
	writer.Fprintln("\nTop files:")
	pathNames := tableEntriesByID(result.StringTables.Paths)
	languageNames := tableEntriesByID(result.StringTables.Languages)
	symbolNamesByID := tableEntriesByID(result.StringTables.SymbolNames)
	compactSymbols := compactSymbolsByID(result.CompactSymbols)
	// Compact results store string and symbol payloads by ID, so text output
	// rebuilds the readable labels on demand before printing each file row.
	for _, file := range result.CompactFiles {
		writer.Fprintf("- %s [%s] %d bytes\n", pathNames[file.PathID], languageNames[file.LanguageID], file.Bytes)
		names := make([]string, 0, len(file.SymbolRefs))
		for _, symbolID := range file.SymbolRefs {
			symbol, ok := compactSymbols[symbolID]
			if !ok {
				continue
			}
			if name := symbolNamesByID[symbol.NameID]; name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			writer.Fprintf("  symbols: %s\n", strings.Join(names, ", "))
		}
	}
}

func buildIndexMarkdownDocument(result indexer.Result) markdownDocument {
	document := markdownDocument{
		Title: "Index Result",
		Sections: []markdownSection{
			{
				Heading: "Summary",
				Items:   buildIndexSummaryItems(result),
			},
		},
	}
	if fileSection := buildIndexFilesSection(result); len(fileSection.Items) > 0 {
		document.Sections = append(document.Sections, fileSection)
	}
	if dependencySection := buildStringSection("Dependencies", result.Dependencies, func(dep indexer.ModuleDependency) string {
		return "Path: " + markdownCode(dep.Path)
	}); len(dependencySection.Items) > 0 {
		document.Sections = append(document.Sections, dependencySection)
	}
	if warningSection := buildStringSection("Warnings", result.Warnings, func(warning string) string {
		return warning
	}); len(warningSection.Items) > 0 {
		document.Sections = append(document.Sections, warningSection)
	}
	if notesSection := buildStringSection("Notes", result.ToolHints, func(note string) string {
		return note
	}); len(notesSection.Items) > 0 {
		document.Sections = append(document.Sections, notesSection)
	}
	return document
}

// buildSearchFilesMarkdownDocument shapes command output into a presentation
// model first, so Markdown rendering stays shared across commands.
func buildSearchFilesMarkdownDocument(output searchFilesOutput) markdownDocument {
	return markdownDocument{
		Title: "File Search",
		Sections: []markdownSection{
			{
				Items: []markdownListItem{
					{Text: "Query: " + markdownCode(output.Query)},
					{Text: fmt.Sprintf("Total matches: %s", markdownCode(fmt.Sprintf("%d", output.TotalMatches)))},
				},
			},
			{
				Heading: "Files",
				Items:   markdownFileMatchItems(output.Files),
			},
		},
	}
}

func buildSearchSymbolsMarkdownDocument(output searchSymbolsOutput) markdownDocument {
	items := make([]markdownListItem, 0, len(output.Symbols))
	for _, symbol := range output.Symbols {
		items = append(items, markdownListItem{
			Text: fmt.Sprintf(
				"Name: %s, kind: %s, path: %s, lines: %s",
				markdownCode(symbol.Name),
				markdownCode(symbol.Kind),
				markdownCode(symbol.Path),
				markdownCode(fmt.Sprintf("%d-%d", symbol.StartLine, symbol.EndLine)),
			),
		})
	}
	return markdownDocument{
		Title: "Symbol Search",
		Sections: []markdownSection{
			{
				Items: []markdownListItem{
					{Text: "Query: " + markdownCode(output.Query)},
					{Text: fmt.Sprintf("Total matches: %s", markdownCode(fmt.Sprintf("%d", output.TotalMatches)))},
				},
			},
			{
				Heading: "Symbols",
				Items:   items,
			},
		},
	}
}

func buildSearchWarningsMarkdownDocument(output searchWarningsOutput) markdownDocument {
	return markdownDocument{
		Title: "Warning Search",
		Sections: []markdownSection{
			{
				Items: []markdownListItem{
					{Text: "Query: " + markdownCode(output.Query)},
					{Text: fmt.Sprintf("Total matches: %s", markdownCode(fmt.Sprintf("%d", output.TotalMatches)))},
				},
			},
			buildStringSection("Warnings", output.Warnings, func(warning string) string {
				return warning
			}),
		},
	}
}

func buildShowFileMarkdownDocument(output showFileOutput) markdownDocument {
	return markdownDocument{
		Title: "File Detail",
		Sections: []markdownSection{
			{
				Items: []markdownListItem{
					{Text: "Requested path: " + markdownCode(output.Path)},
					{Text: "Package: " + markdownCode(output.Match.Package)},
				},
			},
			{
				Heading: "Files",
				Items:   markdownFileSummaryItems([]indexer.FileSummary{output.Match.File}),
			},
		},
	}
}

func buildShowPackageMarkdownDocument(output showPackageOutput) markdownDocument {
	return markdownDocument{
		Title: "Package Detail",
		Sections: []markdownSection{
			{
				Items: []markdownListItem{
					{Text: "Requested name: " + markdownCode(output.Name)},
					{Text: "File count: " + markdownCode(fmt.Sprintf("%d", output.Match.Package.FileCount))},
					{Text: "Symbol count: " + markdownCode(fmt.Sprintf("%d", output.Match.Package.SymbolCount))},
				},
			},
			{
				Heading: "Files",
				Items:   markdownFileSummaryItems(output.Match.Files),
			},
		},
	}
}

func buildIndexSummaryItems(result indexer.Result) []markdownListItem {
	items := []markdownListItem{
		{Text: "Indexer: " + markdownCode(result.Indexer)},
		{Text: "Root: " + markdownCode(result.Root)},
		{Text: "Files scanned: " + markdownCode(fmt.Sprintf("%d", result.FilesScanned))},
		{Text: "Files indexed: " + markdownCode(fmt.Sprintf("%d", result.FilesIndexed))},
		{Text: "Files skipped: " + markdownCode(fmt.Sprintf("%d", result.FilesSkipped))},
	}
	if result.Language != "" {
		items = append(items, markdownListItem{Text: "Language: " + markdownCode(result.Language)})
	}
	if result.SCIPPath != "" {
		items = append(items, markdownListItem{Text: "SCIP path: " + markdownCode(result.SCIPPath)})
	}
	if result.NextPageToken != "" {
		items = append(items, markdownListItem{Text: "Next page token: " + markdownCode(result.NextPageToken)})
	}
	return items
}

func buildIndexFilesSection(result indexer.Result) markdownSection {
	if len(result.FileSummaries) > 0 {
		return markdownSection{
			Heading: "Files",
			Items:   markdownFileSummaryItems(result.FileSummaries),
		}
	}
	if len(result.CompactFiles) > 0 {
		return markdownSection{
			Heading: "Files",
			Items:   markdownCompactFileItems(result),
		}
	}
	return markdownSection{}
}

func markdownFileSummaryItems(files []indexer.FileSummary) []markdownListItem {
	items := make([]markdownListItem, 0, len(files))
	for _, file := range files {
		item := markdownListItem{
			Text: fmt.Sprintf(
				"Path: %s, language: %s, bytes: %s",
				markdownCode(file.Path),
				markdownCode(file.Language),
				markdownCode(fmt.Sprintf("%d", file.Bytes)),
			),
		}
		if names := symbolNames(file.Symbols); len(names) > 0 {
			item.Children = append(item.Children, "Symbols: "+joinMarkdownCodes(names))
		}
		if file.SkipReason != "" {
			item.Children = append(item.Children, "Skip reason: "+markdownCode(file.SkipReason))
		}
		items = append(items, item)
	}
	return items
}

func markdownFileMatchItems(files []indexer.FileMatch) []markdownListItem {
	items := make([]markdownListItem, 0, len(files))
	for _, match := range files {
		item := markdownListItem{
			Text: fmt.Sprintf(
				"Path: %s, package: %s, language: %s, bytes: %s",
				markdownCode(match.File.Path),
				markdownCode(match.Package),
				markdownCode(match.File.Language),
				markdownCode(fmt.Sprintf("%d", match.File.Bytes)),
			),
		}
		if names := symbolNames(match.File.Symbols); len(names) > 0 {
			item.Children = append(item.Children, "Symbols: "+joinMarkdownCodes(names))
		}
		items = append(items, item)
	}
	return items
}

func markdownCompactFileItems(result indexer.Result) []markdownListItem {
	pathNames := tableEntriesByID(result.StringTables.Paths)
	languageNames := tableEntriesByID(result.StringTables.Languages)
	symbolNamesByID := tableEntriesByID(result.StringTables.SymbolNames)
	compactSymbols := compactSymbolsByID(result.CompactSymbols)

	// Compact file rows rebuild the human-readable strings lazily from the
	// dictionary tables emitted by the indexer.
	items := make([]markdownListItem, 0, len(result.CompactFiles))
	for _, file := range result.CompactFiles {
		item := markdownListItem{
			Text: fmt.Sprintf(
				"Path: %s, language: %s, bytes: %s",
				markdownCode(pathNames[file.PathID]),
				markdownCode(languageNames[file.LanguageID]),
				markdownCode(fmt.Sprintf("%d", file.Bytes)),
			),
		}
		names := make([]string, 0, len(file.SymbolRefs))
		for _, symbolID := range file.SymbolRefs {
			symbol, ok := compactSymbols[symbolID]
			if !ok {
				continue
			}
			if name := symbolNamesByID[symbol.NameID]; name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			item.Children = append(item.Children, "Symbols: "+joinMarkdownCodes(names))
		}
		items = append(items, item)
	}
	return items
}

func buildStringSection[T any](heading string, values []T, render func(T) string) markdownSection {
	items := make([]markdownListItem, 0, len(values))
	for _, value := range values {
		items = append(items, markdownListItem{Text: render(value)})
	}
	return markdownSection{
		Heading: heading,
		Items:   items,
	}
}

func joinMarkdownCodes(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, markdownCode(value))
	}
	return strings.Join(quoted, ", ")
}

// symbolNames extracts and sorts symbol names for stable text and Markdown output.
func symbolNames(symbols []indexer.Symbol) []string {
	names := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		if symbol.Name != "" {
			names = append(names, symbol.Name)
		}
	}
	slices.Sort(names)
	return names
}

// tableEntriesByID rebuilds a string lookup map from compact result tables.
func tableEntriesByID(entries []indexer.StringTableEntry) map[int]string {
	values := make(map[int]string, len(entries))
	for _, entry := range entries {
		values[entry.ID] = entry.Value
	}
	return values
}

// compactSymbolsByID rebuilds a symbol lookup map for compact file rendering.
func compactSymbolsByID(symbols []indexer.CompactSymbol) map[int]indexer.CompactSymbol {
	values := make(map[int]indexer.CompactSymbol, len(symbols))
	for _, symbol := range symbols {
		values[symbol.ID] = symbol
	}
	return values
}
