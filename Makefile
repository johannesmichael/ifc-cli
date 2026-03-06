.DEFAULT_GOAL := all

.PHONY: all build test lint bench clean release

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
	rm -rf bin/ dist/

# Cross-platform release builds
# TODO: Enable CGO when duckdb is added (requires platform-specific C toolchains)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

release: clean
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*}; \
		GOARCH=$${platform#*/}; \
		BINARY=ifc-to-db-$${GOOS}-$${GOARCH}; \
		if [ "$${GOOS}" = "windows" ]; then BINARY="$${BINARY}.exe"; fi; \
		echo "Building $${BINARY}..."; \
		CGO_ENABLED=0 GOOS=$${GOOS} GOARCH=$${GOARCH} \
			go build -trimpath -ldflags="-s -w" -o dist/$${BINARY} ./cmd/ifc-to-db || exit 1; \
	done
	@cd dist && sha256sum * > checksums-sha256.txt
	@echo "Release binaries in dist/"
