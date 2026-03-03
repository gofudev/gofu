VERSION ?= $(shell git describe --tags --match 'v*' --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build install test fmt vet lint

build:
	go build $(LDFLAGS) -o bin/gofu ./cmd/gofu/

install:
	go install ./cmd/gofu/

test:
	go test ./...

fmt:
	find . -name '*.go' -not -path '*/testdata/unparseable/*' | xargs gofmt -w

vet:
	go vet ./...

lint:
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping"
