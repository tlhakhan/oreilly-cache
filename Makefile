.PHONY: build build-linux-amd64 build-linux-arm64 test run clean web-install web-dev web-build web-check build-all run-with-web

BIN := oreilly-cache

build: build-linux-amd64 build-linux-arm64

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o $(BIN)-linux-amd64 ./cmd/oreilly-cache/...

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(BIN)-linux-arm64 ./cmd/oreilly-cache/...

test:
	go test ./...

run:
	go run ./cmd/oreilly-cache/...

clean:
	rm -f $(BIN)-linux-amd64 $(BIN)-linux-arm64

web-install:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

web-check:
	cd web && npx biome check . && npx tsc --noEmit && npx vitest run

build-all: build web-build

run-with-web: web-build
	go run ./cmd/oreilly-cache/... -static-dir ./web/dist
