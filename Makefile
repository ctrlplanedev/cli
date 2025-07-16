VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse HEAD)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -X github.com/ctrlplanedev/cli/cmd/ctrlc/root/version.Version=$(VERSION) \
          -X github.com/ctrlplanedev/cli/cmd/ctrlc/root/version.GitCommit=$(COMMIT) \
          -X github.com/ctrlplanedev/cli/cmd/ctrlc/root/version.BuildDate=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/ctrlc ./cmd/ctrlc

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/ctrlc

test:
	go test -v ./...

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...

format:
	go fmt ./...

.PHONY: build install test clean lint format