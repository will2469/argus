export GOWORK ?= off

.PHONY: all test test-race lint build clean setup-hooks

all: lint test build

setup-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/*
	@echo " Argus git hooks installed to .githooks"

test:
	go test -v ./...

test-race:
	go test -race -v ./...

lint:
	@echo "Running go vet..."
	go vet ./...
	@echo "Checking gofmt..."
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*' -not -path '*/testdata/*'))" || (echo " Unformatted files found. Run gofmt -w ." && exit 1)
	@echo " Code hygiene clean!"

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)

build:
	go build -ldflags="$(LDFLAGS)" -v -o bin/argus ./cmd/argus

format:
	@if [ -x .githooks/format-all.sh ]; then \
		.githooks/format-all.sh; \
	else \
		gofmt -w $$(find . -name '*.go' -not -path './vendor/*' -not -path '*/testdata/*'); \
	fi

clean:
	rm -rf bin/ coverage.txt coverage.html
