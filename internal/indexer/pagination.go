package indexer

import paging "github.com/vjanelle/scip-cli/internal/indexer/paging"

type pageCursor struct {
	FileOffset int `json:"fileOffset"`
	DepOffset  int `json:"depOffset"`
}

// PaginateResult slices file and dependency results using the request's page
// parameters and emits a continuation token when more data remains.
func PaginateResult(result Result, req IndexRequest) (Result, error) {
	return paging.PaginateResult(result, req)
}

// decodePageToken parses the opaque pagination cursor returned by a previous
// response.
func decodePageToken(token string) (pageCursor, error) {
	cursor, err := paging.DecodePageToken(token)
	if err != nil {
		return pageCursor{}, err
	}
	return pageCursor(cursor), nil
}

// encodePageToken serializes a pagination cursor for round-tripping through the
// CLI result interface.
func encodePageToken(cursor pageCursor) (string, error) {
	return paging.EncodePageToken(paging.Cursor(cursor))
}
