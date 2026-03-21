package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"
)

var simpleSymbolPattern = regexp.MustCompile(`(?m)^\s*(?:(?:pub)\s+)?(?:func|function|type|class|interface|struct|enum|def|fn)\s+([A-Za-z_][A-Za-z0-9_]*)`)

func buildSExpression(path string, symbols []Symbol) string {
	parts := make([]string, 0, len(symbols)+1)
	parts = append(parts, fmt.Sprintf("(file %q", filepath.ToSlash(path)))
	for _, symbol := range symbols {
		parts = append(parts, fmt.Sprintf(" (sym %q %q %d %d)", symbol.Kind, symbol.Name, symbol.StartLine, symbol.EndLine))
	}
	return strings.Join(parts, "") + ")"
}

func formatSymbol(language string, symbol Symbol, index int) string {
	descriptor := scip.Symbol{
		Scheme: "scip-cli",
		Package: &scip.Package{
			Manager: "local",
			Name:    language,
			Version: ".",
		},
		Descriptors: []*scip.Descriptor{
			{Name: filepath.ToSlash(symbol.Path), Suffix: scip.Descriptor_Namespace},
			{Name: symbol.Name, Disambiguator: fmt.Sprintf("%d", index), Suffix: descriptorSuffix(symbol.Kind)},
		},
	}
	return scip.VerboseSymbolFormatter.FormatSymbol(&descriptor)
}

func descriptorSuffix(kind string) scip.Descriptor_Suffix {
	switch kind {
	case "function":
		return scip.Descriptor_Method
	case "type", "enum":
		return scip.Descriptor_Type
	default:
		return scip.Descriptor_Term
	}
}

func symbolKind(kind string) scip.SymbolInformation_Kind {
	switch kind {
	case "function":
		return scip.SymbolInformation_Function
	case "type":
		return scip.SymbolInformation_Class
	case "enum":
		return scip.SymbolInformation_Enum
	case "variable":
		return scip.SymbolInformation_Variable
	default:
		return scip.SymbolInformation_UnspecifiedKind
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func writeSCIPSnapshot(path, root, toolName string, docs []*scip.Document) error {
	index := &scip.Index{
		Metadata: &scip.Metadata{
			Version: scip.ProtocolVersion_UnspecifiedProtocolVersion,
			ToolInfo: &scip.ToolInfo{
				Name:      toolName,
				Version:   "0.1.0",
				Arguments: []string{},
			},
			ProjectRoot:          "file://" + filepath.ToSlash(root),
			TextDocumentEncoding: scip.TextEncoding_UTF8,
		},
		Documents: docs,
	}

	data, err := proto.Marshal(index)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
