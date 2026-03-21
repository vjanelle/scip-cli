package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type GoIndexer struct {
	goBinary   string
	scipBinary string
	runCommand func(ctx context.Context, dir, name string, args ...string) ([]byte, error)
}

const maxCommandOutputBytes = 1 << 20

var defaultCommandTimeout = 2 * time.Minute

type goIndexCommand struct{}

func init() {
	RegisterCommand(goIndexCommand{})
}

// NewGoIndexer builds the Go-specific backend that shells out to scip-go.
func NewGoIndexer() *GoIndexer {
	scipBinary := "scip-go"
	if goPath := os.Getenv("GOPATH"); goPath != "" {
		binaryName := "scip-go"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}
		scipBinary = filepath.Join(goPath, "bin", binaryName)
	}

	return &GoIndexer{
		goBinary:   "go",
		scipBinary: scipBinary,
		runCommand: runCommand,
	}
}

func (goIndexCommand) Name() string {
	return "go"
}

func (goIndexCommand) Supports(language string, _ IndexRequest) bool {
	return language == "go"
}

func (goIndexCommand) Execute(ctx context.Context, req IndexRequest) (Result, error) {
	return NewGoIndexer().Index(ctx, req)
}

// Index runs scip-go for the requested workspace and enriches the result with
// dependency metadata and sampled file summaries.
func (g *GoIndexer) Index(ctx context.Context, req IndexRequest) (Result, error) {
	req, err := NormalizeRequest(req)
	if err != nil {
		return Result{}, err
	}

	outputPath := req.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join(req.Root, "index.scip")
	}

	args := []string{
		"--output", outputPath,
		"--project-root", ".",
		"--module-root", ".",
		"./...",
	}
	output, err := g.runCommand(ctx, req.Root, g.scipBinary, args...)
	if err != nil {
		return Result{}, fmt.Errorf("run scip-go: %w", err)
	}

	summaries, summaryWarnings := collectGoSummaries(req)
	deps, depErr := g.loadDependencies(ctx, req.Root)
	result := Result{
		Indexer:       "scip-go",
		Root:          req.Root,
		Language:      "go",
		FilesScanned:  len(summaries),
		FilesIndexed:  len(summaries),
		SCIPPath:      outputPath,
		Dependencies:  deps,
		CommandLine:   append([]string{g.scipBinary}, args...),
		FileSummaries: summaries,
	}
	if len(output) > 0 {
		result.Warnings = append(result.Warnings, strings.TrimSpace(string(output)))
	}
	result.Warnings = append(result.Warnings, summaryWarnings...)
	if depErr != nil {
		result.Warnings = append(result.Warnings, "dependency scan failed: "+depErr.Error())
	}

	return result, nil
}

// collectGoSummaries prepares lightweight file-level summaries for paged
// results without parsing Go syntax directly.
func collectGoSummaries(req IndexRequest) ([]FileSummary, []string) {
	files, warnings, err := CollectFiles(IndexRequest{
		Root:          req.Root,
		Language:      "go",
		Paths:         req.Paths,
		SampleLimit:   req.SampleLimit,
		MaxFileBytes:  req.MaxFileBytes,
		IncludeHidden: req.IncludeHidden,
	})
	if err != nil {
		return nil, []string{err.Error()}
	}
	if len(files) > req.SampleLimit {
		files = files[:req.SampleLimit]
	}

	summaries := make([]FileSummary, 0, len(files))
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(req.Root, rel))
		if err != nil {
			warnings = append(warnings, "failed to inspect "+filepath.ToSlash(rel)+": "+err.Error())
			continue
		}
		if warning := InvisibleUnicodeWarning(rel, content); warning != "" {
			warnings = append(warnings, warning)
		}
		summaries = append(summaries, FileSummary{
			Path:     rel,
			Language: "go",
			Bytes:    int64(len(content)),
		})
	}

	return summaries, warnings
}

// loadDependencies queries the active Go module graph with go list -m.
func (g *GoIndexer) loadDependencies(ctx context.Context, root string) ([]ModuleDependency, error) {
	raw, err := g.runCommand(ctx, root, g.goBinary, "list", "-m", "-json", "all")
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	deps := make([]ModuleDependency, 0, 16)
	for decoder.More() {
		var item struct {
			Path     string
			Version  string
			Main     bool
			Indirect bool
			Dir      string
			Replace  *struct {
				Path    string
				Version string
				Dir     string
			}
		}
		if err := decoder.Decode(&item); err != nil {
			return nil, err
		}

		dep := ModuleDependency{
			Path:     item.Path,
			Version:  item.Version,
			Main:     item.Main,
			Indirect: item.Indirect,
			Dir:      item.Dir,
		}
		if item.Replace != nil {
			dep.Replace = item.Replace.Path
			if dep.Dir == "" {
				dep.Dir = item.Replace.Dir
			}
			if dep.Version == "" {
				dep.Version = item.Replace.Version
			}
		}
		deps = append(deps, dep)
	}

	return deps, nil
}

// runCommand executes an external command and returns its combined output.
func runCommand(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultCommandTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output := newLimitedBuffer(maxCommandOutputBytes)
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	data := output.Bytes()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("command timed out: %w", ctx.Err())
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("%s", strings.TrimSpace(string(data)))
		}
		return nil, err
	}

	return data, nil
}

type limitedBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	remaining int
	truncated bool
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{remaining: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	written := len(p)
	if b.remaining <= 0 {
		b.truncated = true
		return written, nil
	}

	if len(p) > b.remaining {
		_, _ = b.buf.Write(p[:b.remaining])
		b.remaining = 0
		b.truncated = true
		return written, nil
	}

	_, _ = b.buf.Write(p)
	b.remaining -= len(p)
	return written, nil
}

func (b *limitedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	data := append([]byte(nil), b.buf.Bytes()...)
	if b.truncated {
		data = append(data, []byte("\n[output truncated]")...)
	}
	return data
}
