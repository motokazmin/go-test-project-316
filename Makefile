.PHONY: build test run help

BIN_DIR = bin
BIN_NAME = hexlet-go-crawler
BIN_PATH = $(BIN_DIR)/$(BIN_NAME)

help:
	@echo "Available commands:"
	@echo "  make build        - Build the crawler executable"
	@echo "  make test         - Run all tests"
	@echo "  make run URL=<url> - Run the crawler with the specified URL"
	@echo "  make clean        - Remove build artifacts"

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_PATH) ./cmd/hexlet-go-crawler

test:
	go test -v -count=1 ./...

run:
	@if [ -z "$(URL)" ]; then \
		echo "Error: URL parameter is required"; \
		echo "Usage: make run URL=https://example.com"; \
		exit 0; \
	fi
	go run ./cmd/hexlet-go-crawler -- $(URL)

clean:
	rm -rf $(BIN_DIR)

