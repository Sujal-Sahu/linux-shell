BINARY=debugshell
GO=go

.PHONY: build run install clean deps

## Download dependencies
deps:
	$(GO) mod tidy
	$(GO) mod download

## Build the binary
build: deps
	$(GO) build -o $(BINARY) .
	@echo "✓ Built: ./$(BINARY)"

## Build with optimizations (smaller binary)
build-prod: deps
	$(GO) build -ldflags="-s -w" -o $(BINARY) .
	@echo "✓ Built (optimized): ./$(BINARY)"

## Run directly without building
run: deps
	$(GO) run .

## Install to system PATH
install: build
	sudo cp $(BINARY) /usr/local/bin/$(BINARY)
	@echo "✓ Installed to /usr/local/bin/$(BINARY)"

## Uninstall from system PATH
uninstall:
	sudo rm -f /usr/local/bin/$(BINARY)
	@echo "✓ Removed /usr/local/bin/$(BINARY)"

## Remove build artifacts
clean:
	rm -f $(BINARY)
	@echo "✓ Cleaned"
