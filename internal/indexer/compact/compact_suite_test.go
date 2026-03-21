package compact

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCompact(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Compact Suite")
}
