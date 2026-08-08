GO ?= go
.PHONY: build test vet fmt all
all: build
build:
	$(GO) build -o bin/hetu ./cmd/hetu
	$(GO) build -o bin/hetud ./cmd/hetud
test:
	$(GO) test ./...
vet:
	$(GO) vet ./...
