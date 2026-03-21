# Development

## Toolchain

- Go 1.26
- GNU Make
- Ginkgo and Gomega for tests
- Optional C compiler such as `gcc` for cgo-backed tree-sitter builds

## Standard workflow

Use the Makefile for all common tasks.

### Bootstrap

```bash
make bootstrap
```

This installs and tidies module dependencies, installs `ginkgo`, installs `scip-go`, and refreshes Go-managed tooling used by the Makefile.

For Zig LSP support with `scip-lsp`, install `zls` with:

```bash
make install-zls
```

### Format

```bash
make fmt
```

### Fix

```bash
make fix
```

### Lint

```bash
make lint
```

### Security

```bash
make security
```

### Test

```bash
make test
```

### Build

```bash
make build
```

### Run

```bash
make run
```

## cgo-backed builds

The repository also exposes cgo-specific targets:

```bash
make build-cgo
make test-cgo
make run-cgo
```

This expects a usable C compiler. If your compiler lives elsewhere, override `CC` when invoking `make`.

## Current cgo note

The cgo-specific path may still fail if the local compiler or headers are incomplete even when `gcc` exists. The default non-cgo workflow remains the supported baseline path for this repository.

## Testing approach

Tests are written with Ginkgo and Gomega.

Current focus areas:

- request normalization
- file collection
- pagination
- compact result encoding and token-budget controls
- Go indexer command behavior
- LSP backend behavior
- CLI parsing and output behavior
- query helpers for file, symbol, warning, and package lookups
- fixture-backed integration coverage across supported languages

Shared language fixtures live under `internal/indexer/testdata/fixtures/polyglot` and currently cover:

- Go
- JavaScript
- TypeScript
- Python
- Rust
- Java
- Zig

The integration suite also asserts that every language returned by `SupportedLanguages()` in `internal/indexer/fs.go` has a matching fixture directory, so adding support for a new language requires extending the fixture corpus.

GitHub automation mirrors the same Makefile entry points:

- `.github/workflows/ci.yml` runs bootstrap, fix, fmt, test, build, and lint
- `.github/workflows/security.yml` runs the vulnerability scan on pushes, pull requests, and a weekly schedule

## Coverage

Recent composite coverage is approximately `80.4%`.

To regenerate coverage manually:

```bash
GOCACHE="$PWD/.gocache" \
GOPATH="$PWD/.gopath" \
GOMODCACHE="$PWD/.gopath/pkg/mod" \
go tool ginkgo -r --cover --coverprofile=coverage.out --randomize-all --randomize-suites --fail-on-pending --keep-going
go tool cover -func ./coverage.out
```

## Implementation notes

### Go indexing

The Go path shells out to `scip-go` and then collects dependency metadata via:

```bash
go list -m -json all
```

### Non-Go indexing

The non-Go path is split into:

- an opt-in `scip-lsp` backend for arbitrary stdio language servers
- a cgo-backed tree-sitter implementation
- a no-cgo symbolic fallback

### Logging

All CLI logging should go to stderr.

## Output shaping

The default response path optimizes for compact, reference-heavy output:

- file and symbol strings are deduplicated through shared string tables
- compact file records reference compact symbol records by ID
- lightweight relationship edges replace repeated structural text
- ranked inline file summaries are trimmed by request budget settings
- pagination can slice both files and dependencies with a continuation token

Useful request knobs:

- `summaryDetail`
- `maxFiles`
- `maxSymbolsPerFile`
- `maxTokensApprox`
- `includeSpans`
- `pageSize`
- `pageToken`

## Source safety checks

The indexers scan source content for suspicious invisible Unicode code points such as zero-width and bidi control characters. When detected, the result includes an explicit warning so review tooling can surface potentially misleading source text.

## Suggested next steps

- stabilize the cgo-backed Windows build path
- extend non-Go symbolic extraction depth
- add higher-level integration tests around emitted SCIP artifacts
