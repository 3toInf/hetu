GO ?= go
.PHONY: build test vet fmt all dist web
all: build
web:
	cd web && npm install && npm run build
build: web
	$(GO) build -o bin/hetu ./cmd/hetu
	$(GO) build -o bin/hetud ./cmd/hetud
test:
	$(GO) test ./...
vet:
	$(GO) vet ./...
dist:
	mkdir -p dist
	@for os in linux darwin; do \
	  for arch in amd64 arm64; do \
	    echo "building $$os/$$arch"; \
	    GOOS=$$os GOARCH=$$arch $(GO) build -o dist/hetu-$$os-$$arch ./cmd/hetu; \
	    GOOS=$$os GOARCH=$$arch $(GO) build -o dist/hetud-$$os-$$arch ./cmd/hetud; \
	  done; \
	done
