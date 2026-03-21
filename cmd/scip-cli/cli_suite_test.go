package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCLI(t *testing.T) {
	// Ginkgo owns the test lifecycle for this package, so the suite entrypoint
	// only needs to register Gomega and hand control over once.
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI Suite")
}
