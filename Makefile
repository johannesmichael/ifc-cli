.DEFAULT_GOAL := all

.PHONY: all build test lint bench clean

all: lint test build

build:
	go build -o bin/ifc-to-db ./cmd/ifc-to-db

test:
	go test ./...

lint:
	golangci-lint run

bench:
	go test -bench=. -benchmem ./...

clean:
	rm -rf bin/
