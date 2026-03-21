package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GoIndexer", func() {
	It("derives the scip-go binary from GOPATH", func() {
		tempGoPath := GinkgoT().TempDir()
		Expect(os.Setenv("GOPATH", tempGoPath)).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv("GOPATH")).To(Succeed())
		})

		indexer := NewGoIndexer()
		Expect(indexer.scipBinary).To(ContainSubstring(filepath.Join(tempGoPath, "bin")))
	})

	It("indexes Go code and collects dependencies", func() {
		tempRoot := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(tempRoot, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644)).To(Succeed())

		indexer := &GoIndexer{
			goBinary:   "go",
			scipBinary: "scip-go",
			runCommand: func(_ context.Context, dir, name string, args ...string) ([]byte, error) {
				Expect(dir).To(Equal(tempRoot))
				switch name {
				case "scip-go":
					Expect(args).To(ContainElement("./..."))
					return []byte("indexed"), nil
				case "go":
					return []byte("{\"Path\":\"example.com/app\",\"Main\":true,\"Dir\":\"" + filepath.ToSlash(tempRoot) + "\"}\n{\"Path\":\"example.com/dep\",\"Version\":\"v1.2.3\",\"Indirect\":true}\n"), nil
				default:
					return nil, fmt.Errorf("unexpected command: %s", name)
				}
			},
		}

		result, err := indexer.Index(context.Background(), IndexRequest{
			Root:        tempRoot,
			Language:    "go",
			SampleLimit: 10,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Indexer).To(Equal("scip-go"))
		Expect(result.Dependencies).To(HaveLen(2))
		Expect(result.FileSummaries).To(HaveLen(1))
		Expect(result.Warnings).To(ContainElement("indexed"))
	})

	It("surfaces scip-go execution failures", func() {
		tempRoot := GinkgoT().TempDir()
		indexer := &GoIndexer{
			goBinary:   "go",
			scipBinary: "scip-go",
			runCommand: func(_ context.Context, _, name string, _ ...string) ([]byte, error) {
				if name == "scip-go" {
					return nil, fmt.Errorf("boom")
				}
				return nil, nil
			},
		}

		_, err := indexer.Index(context.Background(), IndexRequest{Root: tempRoot})
		Expect(err).To(MatchError(ContainSubstring("run scip-go: boom")))
	})

	It("summarizes Go files up to the sample limit", func() {
		tempRoot := GinkgoT().TempDir()
		for index := range 3 {
			name := filepath.Join(tempRoot, fmt.Sprintf("file%d.go", index))
			Expect(os.WriteFile(name, []byte("package sample\n"), 0o644)).To(Succeed())
		}

		summaries, warnings := collectGoSummaries(IndexRequest{
			Root:         tempRoot,
			SampleLimit:  2,
			MaxFileBytes: 1024,
		})

		Expect(warnings).To(BeEmpty())
		Expect(summaries).To(HaveLen(2))
	})

	It("parses replaced dependency metadata", func() {
		indexer := &GoIndexer{
			goBinary: "go",
			runCommand: func(_ context.Context, _, _ string, _ ...string) ([]byte, error) {
				return []byte("{\"Path\":\"example.com/dep\",\"Replace\":{\"Path\":\"../dep\",\"Version\":\"v0.0.1\",\"Dir\":\"C:/dep\"}}\n"), nil
			},
		}

		deps, err := indexer.loadDependencies(context.Background(), ".")
		Expect(err).NotTo(HaveOccurred())
		Expect(deps).To(HaveLen(1))
		Expect(deps[0].Replace).To(Equal("../dep"))
		Expect(deps[0].Dir).To(Equal("C:/dep"))
	})

	It("executes external commands", func() {
		if runtime.GOOS != "windows" {
			Skip("windows-specific shell command")
		}

		output, err := runCommand(context.Background(), ".", "powershell.exe", "-NoProfile", "-Command", "Write-Output hello")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(output)).To(ContainSubstring("hello"))
	})

	It("limits command output size", func() {
		if runtime.GOOS != "windows" {
			Skip("windows-specific shell command")
		}

		output, err := runCommand(context.Background(), ".", "powershell.exe", "-NoProfile", "-Command", "Write-Output ('x' * 1048700)")
		Expect(err).NotTo(HaveOccurred())
		Expect(len(output)).To(BeNumerically("<=", maxCommandOutputBytes+len("\n[output truncated]")))
		Expect(string(output)).To(ContainSubstring("[output truncated]"))
	})

	It("times out commands without a caller deadline", func() {
		if runtime.GOOS != "windows" {
			Skip("windows-specific shell command")
		}

		original := defaultCommandTimeout
		defaultCommandTimeout = 50 * time.Millisecond
		DeferCleanup(func() {
			defaultCommandTimeout = original
		})

		_, err := runCommand(context.Background(), ".", "powershell.exe", "-NoProfile", "-Command", "Start-Sleep -Milliseconds 250")
		Expect(err).To(HaveOccurred())
		Expect(strings.ToLower(err.Error())).To(ContainSubstring("timed out"))
	})
})
