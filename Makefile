.PHONY: build build-linux-amd64 build-linux-arm64 test run clean

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
