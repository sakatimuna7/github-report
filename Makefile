BINARY_NAME=ghreport
INSTALL_PATH=/usr/local/bin

# Versioning
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS = -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)

.PHONY: all build install clean run release publish

all: build

build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .

install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@sudo mv $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Successfully installed $(BINARY_NAME)!"

clean:
	@echo "Cleaning up..."
	@rm -f $(BINARY_NAME)
	@rm -rf dist/

run: build
	@./$(BINARY_NAME)

release:
	@echo "Creating release builds..."
	@mkdir -p dist
	# macOS (Apple Silicon)
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-arm64 .
	# macOS (Intel)
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-amd64 .
	# Linux (AMD64)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64 .
	# Windows (AMD64)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Release builds created in dist/"

publish:
	@bash scripts/publish.sh
