BINARY       := clickhouse-shard-health
MODULE       := github.com/mmtretiak/clickhouse-shard-health
IMAGE        := mmtretiak/clickhouse-shard-health
VERSION      ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS      := -s -w -X main.version=$(VERSION)

.PHONY: build test lint fmt docker clean

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/

test:
	go test -run 'Unit|Integration' -race ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .
	goimports -w .

docker:
	docker build -f build/Dockerfile -t $(IMAGE):$(VERSION) .

clean:
	rm -rf bin/
