VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT  ?= $(shell git rev-parse --short HEAD)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS = -X github.com/dcm-project/cli/internal/version.Version=$(VERSION) \
          -X github.com/dcm-project/cli/internal/version.Commit=$(COMMIT) \
          -X github.com/dcm-project/cli/internal/version.BuildTime=$(BUILD_TIME)

.PHONY: build test test-e2e fmt vet lint clean tidy

build: tidy
	go build -ldflags "$(LDFLAGS)" -o bin/dcm ./cmd/dcm

test: tidy
	go run github.com/onsi/ginkgo/v2/ginkgo -r --randomize-all --randomize-suites --fail-on-pending --keep-going --race --trace ./internal/...

test-e2e: tidy
	go run github.com/onsi/ginkgo/v2/ginkgo -r --randomize-all --randomize-suites --fail-on-pending --keep-going --race --trace --tags=e2e ./test/e2e/...

fmt:
	go fmt ./...

vet: tidy
	go vet ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

tidy:
	go mod tidy
