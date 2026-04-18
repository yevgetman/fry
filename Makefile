MODULE := github.com/yevgetman/fry
BINARY := fry
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build install test lint clean

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/fry/

install:
	go install $(LDFLAGS) ./cmd/fry/

test:
	go test -race ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
