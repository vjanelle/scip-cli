# Architecture

## Goal

`scip-cli` provides a local CLI that indexes source code on demand and can optionally emit a workspace-local SCIP snapshot.

The design prioritizes:

- a straightforward local CLI workflow
- low-token symbolic summaries for large repositories
- a dedicated Go indexing path that can capture Go module dependencies
- an opt-in LSP path for languages that can be served over stdio
- compact structured output that works well for both humans and automation

## Main components

## CLI entrypoint

`cmd/scip-cli`

- defines the `fisk` application
- registers the `index`, `search`, `show`, and `skills` commands
- supports `scip-cli --root .` as shorthand for `scip-cli index --root .`
- writes errors to stderr and shaped results to stdout

## Indexing layer

`internal/indexer`

Responsibilities:

- normalize requests
- collect candidate files
- dispatch between Go, LSP, and fallback indexers
- rank and compact symbolic summaries
- paginate large outputs
- build dictionary-backed output structures and lightweight relationship edges

### Go path

`internal/indexer/go.go`

- shells out to `scip-go`
- collects Go module dependency metadata via `go list -m -json all`
- returns result metadata and sampled file summary stubs

### Non-Go path

Two implementations exist:

- `internal/indexer/treesitter.go`
  - cgo-backed `go-tree-sitter` implementation
  - intended for richer symbolic extraction and optional SCIP snapshot generation
- `internal/indexer/treesitter_nocgo.go`
  - compile-safe fallback when cgo is unavailable
  - provides regex-based symbolic summaries for supported non-Go files

The default build uses the no-cgo path unless cgo-specific targets are selected.

### LSP path

`internal/indexer/lsp.go`

- shells out to a caller-supplied language server command
- speaks LSP over stdio using a small JSON-RPC transport
- turns document symbols and references into SCIP documents
- uses dependency injection so indexing logic can be tested with mocks

## Data flow

1. the CLI parses flags into an `IndexRequest`
2. the request is normalized with stable defaults
3. the dispatcher selects:
   - `scip-go` for Go
   - `scip-lsp` when explicitly requested
   - fallback symbolic indexers for other languages
4. `index` compacts and budget-shapes the result, while `search` and `show` query the fresh in-memory result directly
5. pagination is applied for `index` when requested
6. the CLI writes JSON, Markdown, or text output

## Output model

The primary result type is `internal/indexer/types.go`.

It includes:

- indexer identity
- root and language
- file counts
- ranked file summaries
- optional dependency metadata
- package summaries
- dictionary-encoded string tables
- compact file and symbol tables
- relationship edges
- optional `.scip` output path
- warnings
- pagination metadata and opaque continuation token
- budget metadata describing truncation and omitted fields

## Tradeoffs

- The default build path avoids requiring a working cgo toolchain.
- The Go path is stronger than the non-Go path because it uses `scip-go`.
- The LSP path is flexible, but symbol quality depends on the configured server.
- The non-Go symbolic path stays intentionally compact to keep output volume down.
- The CLI currently favors a single request/response flow rather than background jobs or stored follow-up lookups.
- `search` and `show` rerun indexing for each invocation instead of reading from a persisted result store.
