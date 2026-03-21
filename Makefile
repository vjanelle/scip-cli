SHELL := /usr/bin/env sh

ROOT := $(subst \,/,$(CURDIR))
GO ?= go
CC ?= gcc

ifeq ($(OS),Windows_NT)
EXEEXT := .exe
HOME_DIR ?= $(USERPROFILE)
else
EXEEXT :=
HOME_DIR ?= $(HOME)
endif

BINARY := scip-cli$(EXEEXT)
CMD_DIR := ./cmd/scip-cli
LOCAL_BIN ?= $(subst \,/,$(HOME_DIR))/.local/bin
COVERAGE_MIN ?= 80
LSP_NAME ?= zls
LSP_REPO ?= https://github.com/zigtools/zls.git
LSP_BUILD_DIR ?= $(ROOT)/.tmp/$(LSP_NAME)
LSP_BINARY ?= $(LSP_BUILD_DIR)/zig-out/bin/$(LSP_NAME)$(EXEEXT)
LSP_REF ?=
LSP_BUILD_CMD ?= zig build -Doptimize=ReleaseSafe
ZLS_REF ?= 0.15.1

export GOCACHE := $(ROOT)/.gocache
export GOPATH := $(ROOT)/.gopath
export GOMODCACHE := $(ROOT)/.gopath/pkg/mod

.PHONY: bootstrap fix fmt security test coverage build run install uninstall clean test-cgo build-cgo run-cgo install-lsp install-zls install-scip-go

USER_GOPATH := $(subst \,/,$(HOME_DIR))/go

# Recipes intentionally stick to POSIX shell syntax so `make` behaves
# consistently under bash/sh on Windows and on Unix-like systems.
bootstrap:
	$(GO) get github.com/tree-sitter/go-tree-sitter@latest
	$(GO) get github.com/tree-sitter/tree-sitter-javascript@latest
	$(GO) get github.com/tree-sitter/tree-sitter-typescript@latest
	$(GO) get github.com/tree-sitter/tree-sitter-python@latest
	$(GO) get github.com/tree-sitter/tree-sitter-rust@latest
	$(GO) get github.com/tree-sitter/tree-sitter-java@latest
	$(GO) get -tool github.com/onsi/ginkgo/v2/ginkgo@latest
	$(MAKE) install-scip-go
	$(GO) get -tool golang.org/x/vuln/cmd/govulncheck@latest
	$(GO) mod tidy

install-scip-go:
	GOPATH="$(USER_GOPATH)" $(GO) install github.com/sourcegraph/scip-go/cmd/scip-go@latest

fix:
	$(GO) fix ./...

fmt:
	$(GO) fmt ./...

security:
	$(GO) tool golang.org/x/vuln/cmd/govulncheck ./...

test:
	$(GO) tool ginkgo -r -p --race --randomize-all --randomize-suites --fail-on-pending --keep-going

coverage:
	$(GO) tool ginkgo -r --cover --coverprofile=.coverage.out --randomize-all --randomize-suites --fail-on-pending --keep-going
	$(GO) tool cover -func=.coverage.out
	$(GO) run ./scripts/check_coverage --profile .coverage.out --min $(COVERAGE_MIN)

build:
	$(GO) build -buildvcs=false -o "$(BINARY)" $(CMD_DIR)

run:
	$(GO) run -buildvcs=false $(CMD_DIR) --help

install: build
	mkdir -p "$(LOCAL_BIN)"
	cp -f "$(BINARY)" "$(LOCAL_BIN)/$(BINARY)" || { \
		echo "install failed: $(LOCAL_BIN)/$(BINARY) is likely in use; stop running scip-cli processes and retry."; \
		exit 1; \
	}
	@echo "installed $(BINARY) to $(LOCAL_BIN)/$(BINARY)"

uninstall:
	$(RM) "$(LOCAL_BIN)/$(BINARY)"
	@echo "removed $(LOCAL_BIN)/$(BINARY)"

test-cgo:
	CGO_ENABLED=1 CC="$(CC)" $(GO) tool ginkgo -r -p --race --randomize-all --randomize-suites --fail-on-pending --keep-going

build-cgo:
	CGO_ENABLED=1 CC="$(CC)" $(GO) build -buildvcs=false -o "$(BINARY)" $(CMD_DIR)

run-cgo:
	CGO_ENABLED=1 CC="$(CC)" $(GO) run -buildvcs=false $(CMD_DIR) --help

install-lsp:
	mkdir -p "$(LOCAL_BIN)"
	if [ ! -d "$(LSP_BUILD_DIR)" ]; then git clone --depth 1 "$(LSP_REPO)" "$(LSP_BUILD_DIR)"; fi
	if [ -n "$(LSP_REF)" ]; then \
		git -C "$(LSP_BUILD_DIR)" fetch --depth 1 origin "$(LSP_REF)"; \
		git -C "$(LSP_BUILD_DIR)" checkout --detach FETCH_HEAD; \
	else \
		git -C "$(LSP_BUILD_DIR)" pull --ff-only; \
	fi
	cd "$(LSP_BUILD_DIR)" && $(LSP_BUILD_CMD)
	cp -f "$(LSP_BINARY)" "$(LOCAL_BIN)/$(LSP_NAME)$(EXEEXT)"
	@echo "installed $(LSP_NAME) to $(LOCAL_BIN)/$(LSP_NAME)$(EXEEXT)"

install-zls:
	$(MAKE) install-lsp LSP_NAME=zls LSP_REPO=https://github.com/zigtools/zls.git LSP_BUILD_DIR="$(ROOT)/.tmp/zls" LSP_BINARY="$(ROOT)/.tmp/zls/zig-out/bin/zls$(EXEEXT)" LSP_REF="$(ZLS_REF)" LSP_BUILD_CMD='zig build -Doptimize=ReleaseSafe'

clean:
	$(RM) "$(BINARY)"
