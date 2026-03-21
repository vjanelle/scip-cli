package indexer

import securityhelpers "github.com/vjanelle/scip-cli/internal/indexer/security"

// InvisibleUnicodeWarning remains the stable package entrypoint while the
// Unicode inspection logic lives in a dedicated security helper package.
func InvisibleUnicodeWarning(path string, content []byte) string {
	return securityhelpers.InvisibleUnicodeWarning(path, content)
}
