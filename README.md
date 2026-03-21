# scip-cli

`scip-cli` is a local command-line tool for source indexing and SCIP snapshot generation.

The binary accepts workspace-focused requests, routes them through the Go, LSP, and fallback indexers, and emits structured JSON, LLM-friendly Markdown, or a readable text summary. It also supports targeted `search` and `show` commands for fresh in-process queries over the indexed result.

The local Codex skill files are also embedded into the executable, so the CLI can emit installation instructions and the bundled file contents even when the repository checkout is not available.

## Overview

- Interface: CLI
- Primary Go indexer: `github.com/sourcegraph/scip-go/cmd/scip-go`
- Non-Go fallback: symbolic indexing with an optional cgo-backed tree-sitter path
- Optional LSP backend: `scip-lsp` over stdio
- Logging: stderr only
- Test framework: Ginkgo and Gomega
- CLI parser: `github.com/choria-io/fisk`
- Task runner: `make`

## Repository layout

- `cmd/scip-cli`: CLI entrypoint and output formatting
- `internal/indexer`: request normalization, file selection, backend dispatch, pagination, and result shaping
- `docs`: architecture and development notes
- `skills/scip-cli`: local Codex skill for the CLI workflow

## Requirements

- Go 1.26
- GNU Make
- `scip-go` for the Go indexing path, installed by `make bootstrap`
- `zig` if you want to build and use `zls` via `make install-zls`
- Optional for cgo-backed tree-sitter builds: a working C toolchain such as `gcc`

## Quick start

### 1. Bootstrap the workspace

```bash
make bootstrap
```

### 2. Build the CLI

```bash
make build
```

### 3. Run tests

```bash
make test
```

### 4. Index a workspace

```bash
./scip-cli index --root .
```

The CLI also supports a shorthand form:

```bash
./scip-cli --root . --format text
```

On Windows, the built binary is typically `scip-cli.exe`.

## Core commands

### Show help

```bash
./scip-cli --help
./scip-cli index --help
```

### Show version

```bash
./scip-cli --version
```

### Emit JSON output

```bash
./scip-cli index --root . --language go
```

### Emit text output

```bash
./scip-cli index --root . --format text
```

### Emit Markdown output

```bash
./scip-cli index --root . --format markdown
```

### Search indexed files

```bash
./scip-cli search files --root . --query main --format markdown
```

### Search indexed symbols

```bash
./scip-cli search symbols --root . --query Dispatch
```

### Search warnings

```bash
./scip-cli search warnings --root . --query unicode
```

### Show one file

```bash
./scip-cli show file --root . --path internal/indexer/dispatch.go --format markdown
```

### Show one package

```bash
./scip-cli show package --root . --name internal/indexer
```

### Emit bundled skill install instructions

```bash
./scip-cli skills install-instructions --format markdown
```

### Write a SCIP snapshot

```bash
./scip-cli index --root . --emit-scip --output-path ./index.scip
```

### Use the LSP backend

```bash
./scip-cli index --root . --indexer scip-lsp --lsp-command zls --lsp-arg=--stdio
```

## Important flags

- `--root`
- `--language`
- `--indexer`
- `--path` (repeatable)
- `--sample-limit`
- `--page-size`
- `--page-token`
- `--include-deps`
- `--emit-scip`
- `--output-path`
- `--symbolic-only`
- `--include-hidden`
- `--summary-detail`
- `--max-symbols-per-file`
- `--max-files`
- `--max-tokens-approx`
- `--include-spans`
- `--response-mode`
- `--auto-response-mode`
- `--lsp-command`
- `--lsp-arg` (repeatable)
- `--lsp-env` (repeatable)
- `--lsp-init-options`
- `--format json|markdown|text`
- `--pretty` and `--no-pretty`

## Result model

The CLI emits the same shaped result structure used by the indexing engine:

- indexer identity, root, and language
- file counts
- ranked file summaries
- optional dependency metadata
- compact dictionary-backed tables
- pagination metadata and continuation token
- optional `.scip` output path
- warnings and debug info when requested

- `--format json` returns the full structured result object.
- `--format markdown` returns a sectioned result summary that is easier for LLMs to chunk and quote.
- `--format text` returns a concise human-readable summary derived from either inline file summaries or compact result tables.

For query-style commands:

- `search files` returns file matches with package labels and symbol context
- `search symbols` returns matching symbols with path and line information
- `search warnings` returns matching warning text
- `show file` returns one exact file slice
- `show package` returns one exact package or directory slice
- `skills install-instructions` returns bundled Codex skill installation steps and file contents

## Language routing

- Go requests use `scip-go`
- requests with `--indexer scip-lsp` use the configured language server command
- non-Go requests otherwise use the tree-sitter or symbolic fallback path
- a cgo-backed tree-sitter implementation still exists behind the dedicated cgo Make targets

## Make targets

- `make bootstrap`: install and tidy dependencies, install `ginkgo`, install `scip-go`
- `make fix`: run `go fix ./...`
- `make fmt`: run `go fmt ./...`
- `make lint`: run `golangci-lint`
- `make security`: run `govulncheck`
- `make test`: run the default test suite
- `make build`: build the CLI binary
- `make run`: run the CLI with `--help`
- `make build-cgo`: attempt a cgo-backed build
- `make test-cgo`: attempt cgo-backed tests
- `make run-cgo`: run the cgo-backed CLI with `--help`
- `make install-zls`: clone, build, and install `zls`
- `make clean`: remove the built binary

## cgo and tree-sitter

The repository contains a real `go-tree-sitter` implementation for non-Go indexing, but the default workflow does not depend on cgo because C toolchain setup varies across machines and operating systems.

If your environment supports cgo end to end, use:

```bash
make build-cgo
make test-cgo
```

If cgo is unavailable or incomplete, the default `make build` and `make test` flow remains the supported baseline.

## Testing and coverage

Tests use Ginkgo and Gomega.

The standard path is:

```bash
make test
```

To regenerate coverage manually, use the `ginkgo` binary in your Go bin directory and then inspect the generated profile:

```bash
ginkgo -r --cover --coverprofile=coverage.out --randomize-all --randomize-suites --fail-on-pending --keep-going
go tool cover -func ./coverage.out
```

## Notes

- The supported surface is the local CLI.
- All runtime logging should go to stderr.
- The default build path still prioritizes reproducibility over enabling the cgo-backed tree-sitter path automatically.
- GitHub Actions runs Makefile-driven CI in `.github/workflows/ci.yml` and scheduled vulnerability scans in `.github/workflows/security.yml`.
