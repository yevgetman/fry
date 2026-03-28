build:
	go build -o bin/fry ./cmd/fry

test:
	go test -race ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

PREFIX ?= $(HOME)/.local

install: build
	mkdir -p $(PREFIX)/bin
	cp bin/fry $(PREFIX)/bin/fry
