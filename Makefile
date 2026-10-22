BINARY_NAME := monday
CMD_PATH := ./cmd/monday
INSTALL_DIR := /usr/local/bin
GOARCH ?= $(shell go env GOARCH)
LDFLAGS := -s -w

.PHONY: help build-macos build-linux install clean

help:
	@echo "Targets:"
	@echo "  make build-macos  - build $(BINARY_NAME) for macOS (GOARCH=$(GOARCH), override with GOARCH=amd64|arm64)"
	@echo "  make build-linux  - build $(BINARY_NAME) for Linux (GOARCH=$(GOARCH), override with GOARCH=amd64|arm64)"
	@echo "  make install      - install the built $(BINARY_NAME) binary to $(INSTALL_DIR)"
	@echo "  make clean        - remove the built $(BINARY_NAME) binary"

build-macos:
	GOOS=darwin GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(CMD_PATH)

build-linux:
	GOOS=linux GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(CMD_PATH)

install:
	@test -f $(BINARY_NAME) || { echo "error: $(BINARY_NAME) not found - run 'make build-macos' or 'make build-linux' first"; exit 1; }
	sudo install -m 755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)
