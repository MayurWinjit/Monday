BINARY_NAME := monday
CMD_PATH := ./cmd/monday
INSTALL_DIR := /usr/local/bin
GOARCH ?= $(shell go env GOARCH)
VERSION ?= dev
DIST_DIR := dist
LDFLAGS := -s -w -X github.com/MayurWinjit/monday/internal/buildinfo.Version=$(VERSION)

RELEASE_TARGETS := \
	darwin/amd64 \
	darwin/arm64 \
	linux/amd64 \
	linux/arm64

.PHONY: help build-macos build-linux install release-build lint test clean

help:
	@echo "Targets:"
	@echo "  make build-macos   - build $(BINARY_NAME) for macOS (GOARCH=$(GOARCH), override with GOARCH=amd64|arm64)"
	@echo "  make build-linux   - build $(BINARY_NAME) for Linux (GOARCH=$(GOARCH), override with GOARCH=amd64|arm64)"
	@echo "  make install       - install the built $(BINARY_NAME) binary to $(INSTALL_DIR)"
	@echo "  make release-build - cross-compile darwin/linux amd64+arm64 binaries + checksums.txt into $(DIST_DIR)/ (set VERSION=vX.Y.Z)"
	@echo "  make lint          - run golangci-lint"
	@echo "  make test          - run go vet and go test -race"
	@echo "  make clean         - remove the built $(BINARY_NAME) binary and $(DIST_DIR)/"

build-macos:
	GOOS=darwin GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(CMD_PATH)

build-linux:
	GOOS=linux GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(CMD_PATH)

install:
	@test -f $(BINARY_NAME) || { echo "error: $(BINARY_NAME) not found - run 'make build-macos' or 'make build-linux' first"; exit 1; }
	sudo install -m 755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)

release-build:
	@mkdir -p $(DIST_DIR)
	@for target in $(RELEASE_TARGETS); do \
		os=$$(echo $$target | cut -d/ -f1); \
		arch=$$(echo $$target | cut -d/ -f2); \
		out=$(DIST_DIR)/$(BINARY_NAME)-$$os-$$arch; \
		echo "building $$out"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags="$(LDFLAGS)" -o $$out $(CMD_PATH) || exit 1; \
	done
	@cd $(DIST_DIR) && (command -v sha256sum >/dev/null && sha256sum $(BINARY_NAME)-* > checksums.txt || shasum -a 256 $(BINARY_NAME)-* > checksums.txt)

lint:
	golangci-lint run ./...

test:
	go vet ./...
	go test -race ./...

clean:
	rm -f $(BINARY_NAME)
	rm -rf $(DIST_DIR)
