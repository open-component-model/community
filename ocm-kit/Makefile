# ocm-kit build tooling — self-contained, no external includes.

SHELL := /usr/bin/env bash -o pipefail
.SHELLFLAGS := -ec
MAKEFLAGS += --no-print-directory

BUILD_PATH ?= $(shell pwd)

# Local bin dir for tools installed via `go install`.
LOCALBIN ?= $(BUILD_PATH)/bin
LOCALGOBIN := $(LOCALBIN)/go
$(LOCALGOBIN):
	@mkdir -p $(LOCALGOBIN)

# Toolchain from the dev shell / PATH (overridable).
GO ?= go
DOCKER ?= docker
SHELLCHECK ?= shellcheck

# Tools installed from tools.lock into $(LOCALGOBIN).
GOLANGCI_LINT ?= $(LOCALGOBIN)/golangci-lint
OSV_SCANNER ?= $(LOCALGOBIN)/osv-scanner
# The OCM CLI is pinned in tools.lock under its produced binary name, "cli",
# so the tools.lock rule below builds it. Expose it as "ocm" via a copy.
OCM ?= $(LOCALGOBIN)/ocm
$(OCM): $(LOCALGOBIN)/cli
	@cp -f $< $@


##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## Remove the local tool bin dir
	rm -rf $(LOCALBIN)

##@ Build & test

.PHONY: build
build: ## Build the ocm-kit binary into bin/
	$(GO) build -o $(LOCALBIN)/ocm-kit ./cmd/ocm-kit

.PHONY: fmt
fmt: $(GOLANGCI_LINT) ## Format code
	$(GO) fmt ./...
	$(GOLANGCI_LINT) run --fix

.PHONY: mod
mod: ## Tidy, download and verify modules
	@$(GO) mod tidy
	@$(GO) mod download
	@$(GO) mod verify

.PHONY: test
test: ## Run unit tests with coverage
	$(GO) test -coverprofile=ocm-kit.coverprofile ./...

##@ Lint & scan

.PHONY: lint
lint: golangci-lint shellcheck ## Run all linters

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Run golangci-lint
	$(GOLANGCI_LINT) run

.PHONY: shellcheck
shellcheck: ## Run shellcheck on tracked shell scripts
	$(SHELLCHECK) $$(git ls-files '*.sh')

.PHONY: scan
scan: $(OSV_SCANNER) ## Scan dependencies for known vulnerabilities
	$(OSV_SCANNER) scan source -r .

##@ End-to-end

.PHONY: e2e
e2e: $(OCM) ## Run e2e tests
	OCM=$(OCM) ./test/e2e.sh $(if $(VERSION),--version $(VERSION))

.PHONY: e2e-keep-zot
e2e-keep-zot: $(OCM) ## Run e2e tests, keeping the zot registry running
	OCM=$(OCM) ./test/e2e.sh --keep-zot $(if $(VERSION),--version $(VERSION))

.PHONY: e2e-stop-zot
e2e-stop-zot: ## Stop and remove the zot registry container
	$(DOCKER) stop zot-registry
	$(DOCKER) rm -f zot-registry
	$(DOCKER) volume rm zot-data

# tools.lock maps "<bin-name> <module>@<version>" per line. This pattern rule
# installs any tool by its binary name, e.g. $(LOCALGOBIN)/golangci-lint.
TOOL_LOCK := $(BUILD_PATH)/tools.lock
$(LOCALGOBIN)/%: $(LOCALGOBIN) $(TOOL_LOCK)
	@toolname=$(notdir $@); \
	module=$$(awk "/^$$toolname / {print \$$2}" $(TOOL_LOCK)); \
	test -n "$$module" || { echo "error: $$toolname not found in $(TOOL_LOCK)"; exit 1; }; \
	version=$${module##*@}; \
	{ test -s $@ && grep -q "$$version" $(LOCALGOBIN)/.$$toolname-version 2>/dev/null; } || { \
		echo "Installing $$toolname $$version..."; \
		GOBIN=$(LOCALGOBIN) $(GO) install "$$module" && echo "$$version" > $(LOCALGOBIN)/.$$toolname-version; \
	}

