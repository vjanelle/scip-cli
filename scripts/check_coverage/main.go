package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type fileCoverage struct {
	path    string
	covered int
	total   int
}

var coverageExemptions = map[string]struct{}{
	// The CLI entrypoint is intentionally thin glue around `run`, so enforcing
	// a line-by-line threshold there would mostly reward testing `os.Exit` wiring.
	"github.com/vjanelle/scip-cli/cmd/scip-cli/main.go": {},
}

func main() {
	profilePath := flag.String("profile", ".coverage.out", "Path to the Go coverage profile.")
	minPercent := flag.Float64("min", 80, "Minimum required per-file coverage percentage.")
	flag.Parse()

	if err := run(*profilePath, *minPercent); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(profilePath string, minPercent float64) error {
	files, err := readCoverageProfile(profilePath)
	if err != nil {
		return err
	}

	below := make([]fileCoverage, 0)
	for _, file := range files {
		if isCoverageExempt(file.path) {
			continue
		}
		if coveragePercent(file) < minPercent {
			below = append(below, file)
		}
	}
	if len(below) == 0 {
		fmt.Printf("all files meet the %.1f%% minimum coverage\n", minPercent)
		return nil
	}

	sort.Slice(below, func(i, j int) bool {
		left := coveragePercent(below[i])
		right := coveragePercent(below[j])
		if left == right {
			return below[i].path < below[j].path
		}
		return left < right
	})

	fmt.Fprintf(os.Stderr, "files below %.1f%% coverage:\n", minPercent)
	for _, file := range below {
		fmt.Fprintf(os.Stderr, "- %s: %.1f%%\n", file.path, coveragePercent(file))
	}
	return errors.New("per-file coverage check failed")
}

// readCoverageProfile aggregates statement counts by file so the threshold is
// weighted by the real number of statements instead of raw block counts.
func readCoverageProfile(profilePath string) ([]fileCoverage, error) {
	file, err := os.Open(profilePath)
	if err != nil {
		return nil, fmt.Errorf("open coverage profile: %w", err)
	}
	defer file.Close()

	byPath := map[string]*fileCoverage{}
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if lineNumber == 1 {
			if !strings.HasPrefix(line, "mode:") {
				return nil, fmt.Errorf("parse coverage profile: missing mode header")
			}
			continue
		}
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("parse coverage profile line %d: expected 3 fields", lineNumber)
		}

		numStatements, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("parse coverage profile line %d statements: %w", lineNumber, err)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("parse coverage profile line %d count: %w", lineNumber, err)
		}

		path := strings.SplitN(fields[0], ":", 2)[0]
		entry := byPath[path]
		if entry == nil {
			entry = &fileCoverage{path: path}
			byPath[path] = entry
		}
		entry.total += numStatements
		if count > 0 {
			entry.covered += numStatements
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read coverage profile: %w", err)
	}

	files := make([]fileCoverage, 0, len(byPath))
	for _, file := range byPath {
		files = append(files, *file)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].path < files[j].path
	})
	return files, nil
}

func coveragePercent(file fileCoverage) float64 {
	if file.total == 0 {
		return 100
	}
	return float64(file.covered) * 100 / float64(file.total)
}

func isCoverageExempt(path string) bool {
	path = filepath.ToSlash(path)
	_, ok := coverageExemptions[path]
	return ok
}
