# Include ODC common make targets
DEV_KIT_VERSION := v1.0.10
-include common.mk
common.mk:
	@[ -f .common.mk-download ] || \
		curl --fail -sSL https://raw.githubusercontent.com/opendefensecloud/dev-kit/$(DEV_KIT_VERSION)/common.mk \
		  -o .common.mk-download
	mv .common.mk-download $@
	printf '%s' '$(DEV_KIT_VERSION)' > .common.mk-version
	touch .common.mk-checked

DOCKER ?= docker

# ---------------------------------------------------------------------------
# OCM v2 CLI
#
# The e2e tests drive the OCM v2 CLI (module
# ocm.software/open-component-model/cli). common.mk still ships a v1 OCM
# install target (curl-based, sdk v1); we override the $(OCM) target here to
# build the v2 CLI from source with `go install`. The install produces a binary
# named "cli", which we relocate to $(OCM) (bin/go/ocm).
OCM_V2_VERSION ?= v0.11.0
OCM_V2_MODULE := ocm.software/open-component-model/cli

$(OCM): $(LOCALGOBIN)
	@test -s $(OCM) && grep -q "$(OCM_V2_VERSION)" $(LOCALGOBIN)/.ocm-version 2>/dev/null || { \
		echo "Installing OCM v2 CLI $(OCM_V2_VERSION)..."; \
		GOBIN=$(LOCALGOBIN) $(GO) install $(OCM_V2_MODULE)@$(OCM_V2_VERSION); \
		mv $(LOCALGOBIN)/cli $(OCM); \
		echo "$(OCM_V2_VERSION)" > $(LOCALGOBIN)/.ocm-version; \
	}

.PHONY: fmt
fmt: $(GOLANGCI_LINT) ## Format code
	$(GO) fmt ./...
	$(GOLANGCI_LINT) run --fix

.PHONY: lint
lint: lint-no-golangci golangci-lint ## Lint code

.PHONY: lint-no-golangci
lint-no-golangci: shellcheck ## Run linters but not golangci-lint to exit early in CI/CD pipeline

.PHONY: test
test: ## Run all tests (except E2E)
	$(GO) test -coverprofile=ocm-kit.coverprofile -v ./...

.PHONY: e2e
e2e: $(OCM) ## Run e2e tests
	OCM=$(OCM) ./test/e2e.sh $(if $(VERSION),--version $(VERSION))

.PHONY: e2e-keep-zot
e2e-keep-zot: # Run e2e tests, but keep zot running
	OCM=$(OCM) ./test/e2e.sh --keep-zot $(if $(VERSION),--version $(VERSION))

.PHONY: e2e-stop-zot
e2e-stop-zot: # Stop and remove zot container
	$(DOCKER) stop zot-registry
	$(DOCKER) rm -f zot-registry
	$(DOCKER) volume rm zot-data
