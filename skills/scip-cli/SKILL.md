---
name: scip-cli
description: Use and extend the scip-cli command-line interface for local source indexing. Trigger when the user wants to run `scip-cli` as a CLI, inspect or shape index output, emit `.scip` snapshots, or add CLI commands, flags, tests, or docs for this repository.
---

# SCIP CLI

Treat this repository as a CLI-first project.

- Prefer `cmd/scip-cli` and `internal/indexer` when changing command behavior or indexing internals.
- Use the existing `indexer.IndexRequest -> Dispatch -> PrepareResult -> PaginateResult` flow when adding or changing CLI behavior.
- Keep new CLI code in ASCII and add brief comments around non-obvious request shaping or output formatting logic.

## Core commands

Use these forms first:

```bash
scip-cli index --root .
scip-cli --root . --format text
scip-cli index --root . --emit-scip --output-path ./index.scip
scip-cli search files --root . --query main --format markdown
scip-cli show file --root . --path internal/indexer/dispatch.go
```

Useful flags already supported by the CLI include:

- `--root`
- `--language`
- `--indexer`
- `--path` (repeatable)
- `--format json|markdown|text`
- `--pretty`
- `--emit-scip`
- `--output-path`
- `--summary-detail`
- `--response-mode`
- `--lsp-command`
- `--lsp-arg` (repeatable)
- `--lsp-env` (repeatable)
- `--lsp-init-options`

Available query commands:

- `search files`
- `search symbols`
- `search warnings`
- `show file`
- `show package`

## Development workflow

When changing the CLI:

1. Edit `cmd/scip-cli` first.
2. Reuse `internal/indexer` types instead of creating duplicate request or result structs.
3. Add focused tests in `cmd/scip-cli/main_test.go` for new command parsing or output behavior.
4. Build and test the command package before touching wider repo surfaces.

## Verification

Use the Makefile first:

```bash
make test
make build
```

If the environment blocks writes to the default Go build cache, prefer workspace-local caches rooted in the current repository:

```bash
mkdir -p .gocache/tmp .gopath/pkg/mod
export GOCACHE="$PWD/.gocache"
export GOMODCACHE="$PWD/.gopath/pkg/mod"
export GOTMPDIR="$PWD/.gocache/tmp"
go test ./cmd/scip-cli
go build -buildvcs=false ./cmd/scip-cli
```

Use `--format text` when a human-readable summary is helpful. Use JSON output when another tool or agent needs structured data.
