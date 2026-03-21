package paging

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/vjanelle/scip-cli/internal/indexer/model"
)

// Cursor is the serialized pagination state shared across paged CLI responses.
type Cursor struct {
	FileOffset int `json:"fileOffset"`
	DepOffset  int `json:"depOffset"`
}

// PaginateResult slices file and dependency results using the request's page
// parameters and emits a continuation token when more data remains.
func PaginateResult(result model.Result, req model.IndexRequest) (model.Result, error) {
	cursor, err := DecodePageToken(req.PageToken)
	if err != nil {
		return model.Result{}, err
	}

	totalFiles := max(len(result.CompactFiles), len(result.FileSummaries))
	totalDeps := len(result.Dependencies)

	result.TotalFiles = totalFiles
	result.TotalDeps = totalDeps
	result.PageSize = req.PageSize

	fileStart := min(cursor.FileOffset, totalFiles)
	fileEnd := min(fileStart+req.PageSize, totalFiles)
	depStart := min(cursor.DepOffset, totalDeps)
	depEnd := min(depStart+req.PageSize, totalDeps)

	summaryStart := min(fileStart, len(result.FileSummaries))
	summaryEnd := min(fileEnd, len(result.FileSummaries))
	result.FileSummaries = append([]model.FileSummary(nil), result.FileSummaries[summaryStart:summaryEnd]...)
	result.Dependencies = append([]model.ModuleDependency(nil), result.Dependencies[depStart:depEnd]...)
	result = paginateCompactFiles(result, fileStart, fileEnd)
	result.ReturnedFiles = max(len(result.FileSummaries), len(result.CompactFiles))
	result.ReturnedDeps = len(result.Dependencies)

	next := Cursor{
		FileOffset: fileEnd,
		DepOffset:  depEnd,
	}
	if fileEnd < totalFiles || depEnd < totalDeps {
		token, err := EncodePageToken(next)
		if err != nil {
			return model.Result{}, err
		}
		result.NextPageToken = token
	}

	return result, nil
}

func paginateCompactFiles(result model.Result, fileStart, fileEnd int) model.Result {
	if len(result.CompactFiles) == 0 {
		return result
	}

	fileStart = min(fileStart, len(result.CompactFiles))
	fileEnd = min(fileEnd, len(result.CompactFiles))

	pagedFiles := append([]model.CompactFileSummary(nil), result.CompactFiles[fileStart:fileEnd]...)
	symbolRefs := map[int]struct{}{}
	fileHandles := map[string]struct{}{}
	for _, file := range pagedFiles {
		fileHandles[file.HandleID] = struct{}{}
		for _, ref := range file.SymbolRefs {
			symbolRefs[ref] = struct{}{}
		}
	}

	pagedSymbols := make([]model.CompactSymbol, 0, len(symbolRefs))
	for _, symbol := range result.CompactSymbols {
		if _, ok := symbolRefs[symbol.ID]; ok {
			pagedSymbols = append(pagedSymbols, symbol)
		}
	}

	pagedRelationships := make([]model.Relationship, 0, len(result.Relationships))
	for _, relationship := range result.Relationships {
		if _, ok := fileHandles[relationship.From]; ok {
			pagedRelationships = append(pagedRelationships, relationship)
			continue
		}
		if _, ok := fileHandles[relationship.To]; ok {
			pagedRelationships = append(pagedRelationships, relationship)
		}
	}

	result.CompactFiles = pagedFiles
	result.CompactSymbols = pagedSymbols
	result.Relationships = pagedRelationships
	return result
}

// DecodePageToken parses the opaque pagination cursor returned by a previous
// response.
func DecodePageToken(token string) (Cursor, error) {
	if token == "" {
		return Cursor{}, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Cursor{}, fmt.Errorf("decode page token: %w", err)
	}

	var cursor Cursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return Cursor{}, fmt.Errorf("parse page token: %w", err)
	}
	if cursor.FileOffset < 0 || cursor.DepOffset < 0 {
		return Cursor{}, fmt.Errorf("page token contains negative offsets")
	}

	return cursor, nil
}

// EncodePageToken serializes a pagination cursor for round-tripping through the
// CLI result interface.
func EncodePageToken(cursor Cursor) (string, error) {
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("encode page token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
