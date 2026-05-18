module github.com/vjanelle/scip-cli

go 1.26

toolchain go1.26.0

tool (
	github.com/onsi/ginkgo/v2/ginkgo
	github.com/scip-code/scip-go/cmd/scip-go
	golang.org/x/vuln/cmd/govulncheck
)

require (
	github.com/choria-io/fisk v0.8.0
	github.com/gomarkdown/markdown v0.0.0-20260217112301-37c66b85d6ab
	github.com/onsi/ginkgo/v2 v2.29.0
	github.com/onsi/gomega v1.41.0
	github.com/sourcegraph/scip/bindings/go/scip v0.0.0-20260226120010-b469379fcb42
	github.com/tree-sitter/go-tree-sitter v0.25.0
	github.com/tree-sitter/tree-sitter-java v0.23.5
	github.com/tree-sitter/tree-sitter-javascript v0.25.0
	github.com/tree-sitter/tree-sitter-python v0.25.0
	github.com/tree-sitter/tree-sitter-rust v0.24.2
	github.com/tree-sitter/tree-sitter-typescript v0.23.2
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/alecthomas/kong v1.14.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20260402051712-545e8a4df936 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/scip-code/scip-go v0.2.4 // indirect
	github.com/scip-code/scip/bindings/go/scip v0.7.1 // indirect
	github.com/sourcegraph/beaut v0.0.0-20240611013027-627e4c25335a // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/telemetry v0.0.0-20260409153401-be6f6cb8b1fa // indirect
	golang.org/x/text v0.36.0 // indirect
	golang.org/x/tools v0.44.0 // indirect
	golang.org/x/tools/go/expect v0.1.1-deprecated // indirect
	golang.org/x/tools/go/packages/packagestest v0.1.1-deprecated // indirect
	golang.org/x/tools/go/vcs v0.1.0-deprecated // indirect
	golang.org/x/vuln v1.1.4 // indirect
)
