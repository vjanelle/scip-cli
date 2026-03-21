# Agent Notes

## Coverage Policy

- Logic files should remain at or above `80%` per-file coverage.
- `cmd/scip-cli/main.go` is exempt because it is intentionally thin entrypoint glue around `run`.
- Avoid low-value tests that only restate implementation details such as static strings or trivial formatting branches. When the remaining uncovered lines are negligible, prefer documenting that judgment over writing brittle tests.

## Test Portability

- Keep tests portable across Windows, WSL, and Unix-like environments.
- Build fixture and workspace paths with `filepath.Join` instead of hardcoded separators such as `..\\..\\...`.
- This is especially important in `cmd/scip-cli` and `internal/indexer` tests, where fixture roots should resolve correctly in both native Windows and WSL runs.
