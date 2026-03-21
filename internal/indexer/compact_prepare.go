package indexer

import compactresult "github.com/vjanelle/scip-cli/internal/indexer/compact"

// PrepareResult keeps the legacy indexer entrypoint stable while compact shaping
// now lives in a focused subpackage with the rest of its helper logic.
func PrepareResult(req IndexRequest, result Result) Result {
	return compactresult.PrepareResult(req, result)
}
