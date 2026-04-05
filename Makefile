BINARY_SERVER   = bin/aicarousel
BINARY_SETUP    = bin/aicarousel-setup
BINARY_APIKEY   = bin/aicarousel-apikey
VERSION        ?= dev

.PHONY: build test lint run docker clean checksums

build:
	go build -ldflags="-X main.version=$(VERSION)" -o $(BINARY_SERVER) ./cmd/server
	go build -ldflags="-X main.version=$(VERSION)" -o $(BINARY_SETUP) ./cmd/setup
	go build -ldflags="-X main.version=$(VERSION)" -o $(BINARY_APIKEY) ./cmd/apikey

test:
	go test -race ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	go vet ./...
	golangci-lint run

run:
	go run ./cmd/server

checksums:
	@cd internal/db/migrations && \
	  (sha256sum *.sql 2>/dev/null || shasum -a 256 *.sql) | sed 's/ \*/ /' > CHECKSUMS && \
	  echo "CHECKSUMS regenerated"

docker:
	docker build -t aicarousel-go:$(VERSION) .

clean:
	rm -rf bin/ coverage.out coverage.html
