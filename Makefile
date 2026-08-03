BINARY  := stencil
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"

.PHONY: build test lint clean install release deps

deps:
	go mod tidy
	go mod download

build:
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
	rm -rf dist/

install:
	go install $(LDFLAGS) .

# Cross-compile for common targets
release: clean
	GOOS=linux   GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64    .
	GOOS=linux   GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64    .
	GOOS=darwin  GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64   .
	GOOS=darwin  GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64   .
	GOOS=windows GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe .
