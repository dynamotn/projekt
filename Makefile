BINARY_FOLDER  = $(CURDIR)/bin
DOC_FOLDER     = $(CURDIR)/doc
INSTALL_PATH  ?= $(HOME)/.local/bin
TARGETS        = darwin/amd64 darwin/arm64 darwin/386 darwin/arm linux/amd64 linux/386 linux/arm linux/arm64

# The version is of the format Major.Minor.Patch[-Prerelease][+BuildMetadata]
GIT_COMMIT = $(shell git rev-parse HEAD)
GIT_TAG    = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
GIT_DIRTY  = $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")
BUILD_TIME = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS += -X gitlab.com/dynamo.foss/projekt/internal/version.gitCommit=${GIT_COMMIT}
LDFLAGS += -X gitlab.com/dynamo.foss/projekt/internal/version.gitTreeState=${GIT_DIRTY}
LDFLAGS += -X gitlab.com/dynamo.foss/projekt/internal/version.version=${GIT_TAG}
LDFLAGS += -X gitlab.com/dynamo.foss/projekt/internal/version.buildTime=${BUILD_TIME}
LDFLAGS += $(EXT_LDFLAGS)

.PHONY: t
t:
	@mkdir -p '$(BINARY_FOLDER)'
	go build -ldflags '$(LDFLAGS)' -o '$(BINARY_FOLDER)'/t $(CURDIR)/cmd/t/main.go

.PHONY: b
b:
	@mkdir -p '$(BINARY_FOLDER)'
	go build -ldflags '$(LDFLAGS)' -o '$(BINARY_FOLDER)'/b $(CURDIR)/cmd/b/main.go

.PHONY: projekt
projekt:
	@mkdir -p '$(BINARY_FOLDER)'
	go build -ldflags '$(LDFLAGS)' -o '$(BINARY_FOLDER)'/projekt $(CURDIR)/cmd/projekt/main.go

.PHONY: build
build: t b projekt

.PHONY: clean
clean:
	@rm -rf '$(BINARY_FOLDER)'
	@rm -rf '$(DOC_FOLDER)'

.PHONY: install
install: t b projekt
	@install '$(BINARY_FOLDER)'/* '$(INSTALL_PATH)'

.PHONY: uninstall
uninstall:
	@rm -f '$(INSTALL_PATH)'/projekt '$(INSTALL_PATH)'/t '$(INSTALL_PATH)'/b

.PHONY: doc
doc:
	@rm -rf '${DOC_FOLDER}'
	@mkdir -p '${DOC_FOLDER}'
	go run $(CURDIR)/doc.go

.PHONY: all
all: install doc

.PHONY: test
test:
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

.PHONY: test-coverage
test-coverage: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: lint
lint:
	golangci-lint run

.PHONY: fmt
fmt:
	gofmt -s -w .
	goimports -w -local gitlab.com/dynamo.foss/projekt .

.PHONY: vet
vet:
	go vet ./...

.PHONY: check
check: fmt vet lint test

.PHONY: deps
deps:
	go mod download
	go mod verify

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: info
info:
	@echo "Git Tag:           ${GIT_TAG}"
	@echo "Git Commit:        ${GIT_COMMIT}"
	@echo "Git Tree State:    ${GIT_DIRTY}"
	@echo "Build Time:        ${BUILD_TIME}"

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build          - Build all binaries (t, b, projekt)"
	@echo "  t              - Build t binary"
	@echo "  b              - Build b binary"
	@echo "  projekt        - Build projekt binary"
	@echo "  install        - Build and install binaries to $(INSTALL_PATH)"
	@echo "  uninstall      - Remove installed binaries"
	@echo "  clean          - Remove build artifacts"
	@echo "  test           - Run tests with coverage"
	@echo "  test-coverage  - Run tests and generate HTML coverage report"
	@echo "  lint           - Run golangci-lint"
	@echo "  fmt            - Format code with gofmt and goimports"
	@echo "  vet            - Run go vet"
	@echo "  check          - Run fmt, vet, lint, and test"
	@echo "  deps           - Download and verify dependencies"
	@echo "  tidy           - Tidy go.mod"
	@echo "  doc            - Generate documentation"
	@echo "  all            - Build, install, and generate documentation"
	@echo "  info           - Display build information"
	@echo "  help           - Display this help message"
