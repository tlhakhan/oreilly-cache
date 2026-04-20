.PHONY: build test run

build:
	go build ./cmd/oreilly-cache/...

test:
	go test ./...

run:
	go run ./cmd/oreilly-cache/...
