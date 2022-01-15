DISTDIR = dist

# Utility versions.
GORELEASER_VERSION    = "v1.3.1"
GOLANGCI_LINT_VERSION = "v1.43.0"

# Build variables.
GIT_TREE_STATE=$(if $(shell git status --porcelain),dirty,clean)

.PHONY: bootstrap
# Download and install all go dependencies and tools.
bootstrap: $(RESULTSDIR) bootstrap-go install-tools

.PHONY: bootstrap-go
bootstrap-go:
	go mod download

.PHONY: install-tools
install-tools: install-goreleaser install-golangci-lint

.PHONY: install-goreleaser
install-goreleaser:
	go install github.com/goreleaser/goreleaser@$(GORELEASER_VERSION)

.PHONY: install-golangci-lint
install-golangci-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: test
test:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: lint
lint:
	golangci-lint run --tests=false --timeout 5m --config .golangci.yaml

.PHONY: build
build: install-goreleaser
	GIT_TREE_STATE=$(GIT_TREE_STATE) goreleaser build --rm-dist --single-target

.PHONY: release
release: install-goreleaser
	GIT_TREE_STATE=$(GIT_TREE_STATE) goreleaser release --rm-dist

.PHONY: release-snapshot
release-snapshot: install-goreleaser
	GIT_TREE_STATE=$(GIT_TREE_STATE) goreleaser release --rm-dist --skip-publish --snapshot

.PHONY: clean-dist
clean-dist:
	rm -rf $(DISTDIR)

.PHONY: clean
clean: clean-dist
	rm -f coverage.txt