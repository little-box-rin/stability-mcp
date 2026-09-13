BINARY   := stability-mcp
BUILD_DIR := build

COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -ldflags="-X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: build clean test lint

build: linux/amd64 linux/arm64 darwin/amd64

linux/amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/stability-mcp/

linux/arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 ./cmd/stability-mcp/

darwin/amd64:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 ./cmd/stability-mcp/

clean:
	rm -rf $(BUILD_DIR)

test:
	go test ./...

lint:
	go vet ./...