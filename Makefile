export GOWORK ?= off

.PHONY: all test test-full test-race test-coverage lint build clean setup-hooks format semgrep

all: lint test build

setup-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/*
	@echo " Argus git hooks installed to .githooks"

# Test Cepat: Ringan & instan untuk siklus iterasi harian
test:
	go test -v ./...

# Test Menyeluruh: Lengkap dengan Go Race Detector
test-full:
	go test -race -v ./...

test-race: test-full

# Test Coverage: Menyeluruh dengan profiling kode produksi (rules, runner, shared, cmd)
COVER_PKGS ?= github.com/will2469/argus/rules/...,github.com/will2469/argus/runner/...,github.com/will2469/argus/shared/...,github.com/will2469/argus/cmd/...
test-coverage:
	go test -race -coverpkg=$(COVER_PKGS) -coverprofile=coverage.txt -covermode=atomic ./...
	@go tool cover -func=coverage.txt | tail -n 1
	go tool cover -html=coverage.txt -o coverage.html
	@echo " Coverage report generated at coverage.html"

lint:
	@echo "Running go vet..."
	go vet ./...
	@echo "Checking gofmt..."
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))" || (echo " Unformatted files found. Run gofmt -w ." && exit 1)
	@echo " Code hygiene clean!"
 
semgrep:
	uvx semgrep .

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
		gofmt -w $$(find . -name '*.go' -not -path './vendor/*'); \
	fi

clean:
	rm -rf bin/ coverage.txt coverage.html
