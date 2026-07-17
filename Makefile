GO ?= go
PKGS := ./...

.PHONY: all build vet fmt fmt-check lint test tidy schema clean

all: build vet lint test

build:
	$(GO) build $(PKGS)

vet:
	$(GO) vet $(PKGS)

fmt:
	gofmt -w .

# Fail if any Go file is not gofmt-clean.
fmt-check:
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then \
		echo "gofmt needed on:"; echo "$$files"; exit 1; \
	fi

# golangci-lint if installed, otherwise fall back to gofmt-check + vet.
lint: fmt-check vet
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed; ran fmt-check + vet only"; \
	fi

test:
	$(GO) test $(PKGS)

# Manifest schema validation lives in internal/manifest tests.
schema:
	$(GO) test ./internal/manifest/...

tidy:
	$(GO) mod tidy

clean:
	rm -rf dist bin
