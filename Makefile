build:
	go build -o bin/fry ./cmd/fry

test:
	go test -race ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

install: build
	cp bin/fry /usr/local/bin/fry
