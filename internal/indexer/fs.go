package indexer

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var ignoredDirs = []string{
	".git",
	".hg",
	".svn",
	".idea",
	".vscode",
	"dist",
	"build",
}

var languageByExtension = map[string]string{
	".go":   "go",
	".js":   "javascript",
	".jsx":  "javascript",
	".cjs":  "javascript",
	".mjs":  "javascript",
	".ts":   "typescript",
	".tsx":  "typescript",
	".mts":  "typescript",
	".cts":  "typescript",
	".py":   "python",
	".rs":   "rust",
	".java": "java",
	".zig":  "zig",
}

// CollectFiles walks the requested scope and returns candidate source files.
// Warnings are returned for non-fatal issues such as missing subpaths.
func CollectFiles(req IndexRequest) ([]string, []string, error) {
	targets := req.Paths
	if len(targets) == 0 {
		targets = []string{"."}
	}

	files := make([]string, 0, 64)
	warnings := make([]string, 0)
	seen := map[string]struct{}{}

	for _, target := range targets {
		absolute := filepath.Join(req.Root, target)
		info, err := os.Stat(absolute)
		if err != nil {
			warnings = append(warnings, "skipping missing path "+target)
			continue
		}

		if !info.IsDir() {
			rel, err := filepath.Rel(req.Root, absolute)
			if err == nil && includePath(req, rel, info) {
				files = append(files, rel)
			}
			continue
		}

		err = filepath.WalkDir(absolute, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				warnings = append(warnings, "walk error for "+path+": "+walkErr.Error())
				return nil
			}

			rel, err := filepath.Rel(req.Root, path)
			if err != nil {
				return nil
			}

			if d.IsDir() {
				if rel == "." {
					return nil
				}
				if shouldSkipDir(req, d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}

			info, err := d.Info()
			if err != nil {
				warnings = append(warnings, "stat error for "+rel+": "+err.Error())
				return nil
			}

			if !includePath(req, rel, info) {
				return nil
			}
			if _, ok := seen[rel]; ok {
				return nil
			}
			seen[rel] = struct{}{}
			files = append(files, rel)
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}

	slices.Sort(files)
	return files, warnings, nil
}

func shouldSkipDir(req IndexRequest, name string) bool {
	if !req.IncludeHidden && strings.HasPrefix(name, ".") {
		return true
	}
	return slices.Contains(ignoredDirs, name)
}

func includePath(req IndexRequest, rel string, info fs.FileInfo) bool {
	if !req.IncludeHidden {
		for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
			if strings.HasPrefix(part, ".") {
				return false
			}
		}
	}
	if info.Size() > req.MaxFileBytes {
		return false
	}
	if req.Language == "" {
		return DetectLanguage(rel) != ""
	}
	return DetectLanguage(rel) == req.Language
}

// DetectLanguage maps file extensions to the indexer's supported language IDs.
func DetectLanguage(path string) string {
	return languageByExtension[strings.ToLower(filepath.Ext(path))]
}

// SupportedLanguages returns the normalized language IDs supported by the indexer.
func SupportedLanguages() []string {
	languages := make([]string, 0, len(languageByExtension))
	seen := map[string]struct{}{}
	for _, language := range languageByExtension {
		if _, ok := seen[language]; ok {
			continue
		}
		seen[language] = struct{}{}
		languages = append(languages, language)
	}
	slices.Sort(languages)
	return languages
}
