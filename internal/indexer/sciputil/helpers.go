package sciputil

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"github.com/vjanelle/scip-cli/internal/indexer/model"
	"google.golang.org/protobuf/proto"
)

// SimpleSymbolPattern powers the lightweight fallback symbol extraction path.
var SimpleSymbolPattern = regexp.MustCompile(`(?m)^\s*(?:(?:pub)\s+)?(?:func|function|type|class|interface|struct|enum|def|fn)\s+([A-Za-z_][A-Za-z0-9_]*)`)

// BuildSExpression renders a compact symbolic form that is cheap to include in
// inline responses when we do not want to send full source text.
func BuildSExpression(path string, symbols []model.Symbol) string {
	parts := make([]string, 0, len(symbols)+1)
	parts = append(parts, fmt.Sprintf("(file %q", filepath.ToSlash(path)))
	for _, symbol := range symbols {
		parts = append(parts, fmt.Sprintf(" (sym %q %q %d %d)", symbol.Kind, symbol.Name, symbol.StartLine, symbol.EndLine))
	}
	return strings.Join(parts, "") + ")"
}

// FormatSymbol builds a stable local SCIP symbol ID for an extracted symbol.
func FormatSymbol(language string, symbol model.Symbol, index int) string {
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

// SymbolKind maps the CLI's compact symbol categories onto SCIP kinds.
func SymbolKind(kind string) scip.SymbolInformation_Kind {
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

// FirstNonEmpty returns the first trimmed, non-empty value from a preference list.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// WriteSCIPSnapshot serializes documents to a workspace-local .scip snapshot.
func WriteSCIPSnapshot(path, root, toolName string, docs []*scip.Document) error {
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
