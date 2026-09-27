# Makefile for nmea-go-parser.
#
# The library itself needs no build step: it is a Go module, so a consumer
# imports it and `go build` does the rest. What this file is for is the command
# line tool in ./cmd/nmea-dump, running the checks, and producing release
# binaries for every platform.
#
# Run `make help` for the list of targets.

# The module path, and the import path of the command.
MODULE      := github.com/tamalmaity-dev/nmea-go-parser
BINARY      := nmea-dump
CMD         := ./cmd/$(BINARY)

# The library version, kept in the VERSION file so there is exactly one place
# to bump it. It is compiled into the binary, so a build reports its own
# version without reading anything at runtime.
VERSION     ?= $(shell cat VERSION)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Where a plain `make build` puts the binary. Release builds go to ./dist.
# $(findstring NT,$(OS)) is the portable Windows test: $(OS) is Windows_NT on
# Windows and empty everywhere else.
DIST        := dist
WIN         := $(findstring NT,$(OS))
EXE         := $(if $(WIN),.exe,)
BIN         := $(BINARY)$(EXE)

# ldflags -s -w drop the symbol table and DWARF data, which is about a third
# off the binary. The values are passed as variables rather than baked in as
# constants so they can be set from the command line:
#
#     make build VERSION=0.2.0
LDFLAGS     := -s -w \
	-X 'main.version=$(VERSION)' \
	-X 'main.commit=$(COMMIT)' \
	-X 'main.date=$(DATE)'

# Platforms for `make release`, as GOOS/GOARCH pairs. The list is the set that
# covers desktop, server, and the single-board computers these receivers tend
# to be attached to.
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	linux/arm \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64 \
	freebsd/amd64

# GOFLAGS is deliberately not set to -mod=vendor: there is no vendor directory,
# and the point of the build matrix is to prove the module resolves from the
# proxy exactly as a consumer would get it.

.PHONY: help
help: ## Show this help
	@echo "$(BINARY) $(VERSION) - NMEA 0183 diagnostic tool"
	@echo ""
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the binary for this machine into ./
	@mkdir -p $(dir $(BIN))
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) $(CMD)
	@echo "built ./$(BIN) ($(VERSION), $(COMMIT))"
.PHONY: install
install: ## Install the binary into GOBIN
	go install -trimpath -ldflags "$(LDFLAGS)" $(CMD)

.PHONY: run
run: build ## Build then read a serial port (PORT=COM3 BAUD=9600)
	./$(BINARY) -port $(or $(PORT),COM3) -baud $(or $(BAUD),9600)

.PHONY: demo
demo: build ## Build then decode the bundled sample capture
	./$(BINARY) -file testdata/sample.nmea

.PHONY: test
test: ## Run the tests
	go test -count=1 -timeout 300s ./...

.PHONY: test-race
test-race: ## Run the tests under the race detector (needs a C toolchain)
	go test -count=1 -race -timeout 600s ./...

.PHONY: cover
cover: ## Run the tests and report coverage
	go test -count=1 -timeout 300s -covermode=atomic -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1
	@echo "HTML report: go tool cover -html=coverage.out"

.PHONY: bench
bench: ## Run the benchmarks
	go test -run XXX -bench . -benchmem -timeout 600s ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format the source
	gofmt -w -s .

.PHONY: fmt-check
fmt-check: ## Fail if anything is unformatted
	@unformatted=$$(gofmt -l -s .); \
	if [ -n "$$unformatted" ]; then \
		echo "these files need gofmt:"; echo "$$unformatted"; exit 1; \
	fi
	@echo "gofmt clean"

.PHONY: tidy
tidy: ## Tidy go.mod and go.sum
	go mod tidy

# The version the module reports. Checked by `make check-version` so a release
# cannot ship a VERSION file that disagrees with a git tag.
.PHONY: check-version
check-version: ## Check VERSION against the latest git tag
	@tag=$$(git describe --tags --abbrev=0 2>/dev/null || echo ""); \
	if [ -z "$$tag" ]; then \
		echo "no git tag yet; VERSION is $(VERSION)"; \
	elif [ "$${tag#v}" != "$(VERSION)" ]; then \
		echo "VERSION is $(VERSION) but the latest tag is $$tag"; exit 1; \
	else \
		echo "VERSION $(VERSION) matches tag $$tag"; \
	fi

.PHONY: check
check: fmt-check vet test ## Everything CI runs

.PHONY: release
release: clean ## Cross-compile release binaries and checksums into ./dist
	@mkdir -p $(DIST)
	@set -e; for platform in $(PLATFORMS); do \
		goos=$${platform%/*}; goarch=$${platform#*/}; \
		out="$(DIST)/$(BINARY)_$(VERSION)_$${goos}_$${goarch}"; \
		if [ "$$goos" = "windows" ]; then out="$$out.exe"; fi; \
		echo "building $$out"; \
		GOOS=$$goos GOARCH=$$goarch CGO_ENABLED=0 \
			go build -trimpath -ldflags "$(LDFLAGS)" -o "$$out" $(CMD); \
	done
	@cd $(DIST) && sha256sum * > SHA256SUMS.txt
	@echo ""
	@echo "release artifacts in ./$(DIST):"
	@ls -1sh $(DIST)

.PHONY: install-dist
install-dist: release ## Build a release and copy this machine's binary to ./bin
	@mkdir -p bin
	@goos=$$(go env GOOS); goarch=$$(go env GOARCH); \
	src="$(DIST)/$(BINARY)_$(VERSION)_$${goos}_$${goarch}"; \
	ext=""; [ "$$goos" = "windows" ] && ext=".exe"; \
	cp "$$src" "bin/$(BINARY)$$ext"
	@echo "copied to ./bin/$(BINARY)$$ext"

.PHONY: clean
clean: ## Remove build output
	rm -rf $(DIST) bin $(BIN) coverage.out *.test *.exe
	@echo "cleaned"

.PHONY: version
version: ## Print the version this build would report
	@echo "$(VERSION)"
