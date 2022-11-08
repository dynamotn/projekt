BINARY_FOLDER  = $(CURDIR)/bin
DOC_FOLDER     = $(CURDIR)/doc
INSTALL_PATH  ?= /usr/local/bin
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

.PHONY: clean
clean:
	@rm -rf '$(BINARY_FOLDER)'

.PHONY: install
install: t b projekt
	@install '$(BINARY_FOLDER)'/* '$(INSTALL_PATH)'

.PHONY: doc
doc:
	@rm -rf '${DOC_FOLDER}'
	@mkdir -p '${DOC_FOLDER}'
	go run $(CURDIR)/doc.go

.PHONY: all
all: install clean doc

.PHONY: info
info:
	 @echo "Git Tag:           ${GIT_TAG}"
	 @echo "Git Commit:        ${GIT_COMMIT}"
	 @echo "Git Tree State:    ${GIT_DIRTY}"
