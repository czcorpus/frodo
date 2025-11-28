VERSION := $(shell git describe --tags 2>/dev/null || echo "dev")
BUILD := $(shell date +%FT%T%z)
HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-w -s -X main.version=$(VERSION) -X main.buildDate=$(BUILD) -X main.gitCommit=$(HASH)"

SERVER_BIN := frodo
DICTBUILDER_BIN := mkdict

BIN_DIR := .
DOCS_DIR := docs

.PHONY: all build server devbuild server-dev dictbuilder dictbuilder-dev test swagger clean help

all: build test

build: server dictbuilder

server:
	@$(MAKE) swagger
	go build $(LDFLAGS) -o $(BIN_DIR)/$(SERVER_BIN) ./cmd/server

devbuild: server-dev dictbuilder-dev

server-dev:
	go build $(LDFLAGS) -o $(BIN_DIR)/$(SERVER_BIN) ./cmd/server

dictbuilder:
	go build $(LDFLAGS) -o $(BIN_DIR)/$(DICTBUILDER_BIN) ./cmd/dictbuilder

dictbuilder-dev:
	go build $(LDFLAGS) -o $(BIN_DIR)/$(DICTBUILDER_BIN) ./cmd/dictbuilder

test:
	go test ./...

swagger:
	@echo "Generating swagger docs..."
	@mkdir -p ./docs
	@go install -v github.com/swaggo/swag/cmd/swag@latest
	@swag init --parseDependency -g frodo.go --dir ./cmd/server --output ./docs --parseDepth 10

clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BIN_DIR)/${SERVER_BIN}
	@rm -f $(BIN_DIR)/${DICTBUILDER_BIN}
	@rm -rf $(DOCS_DIR)

deps:
	go mod tidy
	go mod download

test-coverage:
	go test -cover ./...

help:
	@echo "Available targets:"
	@echo "  all           - Run tests and build both binaries (default)"
	@echo "  build         - Build both binaries"
	@echo "  devbuild      - Build both binaries without Swagger docs (faster)"
	@echo "  server        - Build server binary"
	@echo "  dictbuilder   - Build dictbuilder binary"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  swagger       - Generate swagger documentation"
	@echo "  clean         - Clean all build artifacts"
	@echo "  deps          - Install/update dependencies"
	@echo "  help          - Show this help message"
