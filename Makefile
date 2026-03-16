VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X github.com/sift-works/agent/internal/config.Version=$(VERSION)"

.PHONY: build test lint clean

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/sift-agent ./cmd/sift-agent

test:
	go test ./... -v -race

lint:
	go vet ./...

clean:
	rm -rf bin/
