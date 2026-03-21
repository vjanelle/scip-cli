package skills

import (
	"embed"
	"fmt"
	"path"
	"strings"
)

// BundledFile describes one embedded skill file and where Codex expects it to
// be written during installation.
type BundledFile struct {
	RelativePath string `json:"relativePath"`
	InstallPath  string `json:"installPath"`
	Content      string `json:"content"`
}

// BundleInstallInstructions captures the metadata and file payloads needed to
// install one bundled skill without reading from the repository at runtime.
type BundleInstallInstructions struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	InstallDir  string        `json:"installDir"`
	Steps       []string      `json:"steps"`
	Files       []BundledFile `json:"files"`
}

// InstallInstructions is the structured payload returned by the CLI when a
// user asks how to install the bundled skills into Codex.
type InstallInstructions struct {
	CodexHomeHint string                      `json:"codexHomeHint"`
	InstallRoot   string                      `json:"installRoot"`
	Skills        []BundleInstallInstructions `json:"skills"`
}

type bundleSpec struct {
	name        string
	description string
	relativeDir string
	files       []string
}

var (
	//go:embed scip-cli/SKILL.md scip-cli/agents/openai.yaml
	bundledFiles embed.FS

	bundleCatalog = []bundleSpec{
		{
			name:        "scip-cli",
			description: "Use and extend the scip-cli command-line interface for local source indexing.",
			relativeDir: "scip-cli",
			files: []string{
				"scip-cli/SKILL.md",
				"scip-cli/agents/openai.yaml",
			},
		},
	}
)

// InstallableInstructions returns the embedded skill payloads along with
// stable installation guidance that can be emitted from the CLI.
func InstallableInstructions(name string) (InstallInstructions, error) {
	selected := bundleCatalog
	if name != "" {
		match, ok := findBundle(name)
		if !ok {
			return InstallInstructions{}, fmt.Errorf("unknown bundled skill %q", name)
		}
		selected = []bundleSpec{match}
	}

	skills := make([]BundleInstallInstructions, 0, len(selected))
	for _, bundle := range selected {
		files, err := installFiles(bundle)
		if err != nil {
			return InstallInstructions{}, err
		}

		installDir := path.Join("$CODEX_HOME", "skills", bundle.relativeDir)
		skills = append(skills, BundleInstallInstructions{
			Name:        bundle.name,
			Description: bundle.description,
			InstallDir:  installDir,
			Steps: []string{
				fmt.Sprintf("Create `%s` if it does not already exist.", installDir),
				"Write each bundled file to the install path shown below.",
				"Start a new Codex session so the skill is discovered from disk.",
			},
			Files: files,
		})
	}

	return InstallInstructions{
		CodexHomeHint: "$CODEX_HOME should point to your Codex home directory.",
		InstallRoot:   path.Join("$CODEX_HOME", "skills"),
		Skills:        skills,
	}, nil
}

func findBundle(name string) (bundleSpec, bool) {
	for _, bundle := range bundleCatalog {
		if bundle.name == name {
			return bundle, true
		}
	}

	return bundleSpec{}, false
}

func installFiles(bundle bundleSpec) ([]BundledFile, error) {
	files := make([]BundledFile, 0, len(bundle.files))
	for _, name := range bundle.files {
		content, err := bundledFiles.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read embedded skill file %q: %w", name, err)
		}

		relativePath := path.Clean(strings.TrimPrefix(name, bundle.relativeDir+"/"))
		files = append(files, BundledFile{
			RelativePath: relativePath,
			InstallPath:  path.Join("$CODEX_HOME", "skills", bundle.relativeDir, relativePath),
			Content:      string(content),
		})
	}

	return files, nil
}
