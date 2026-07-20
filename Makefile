.PHONY: run test build

run:
	@set -a; . ./.env; set +a; go run ./cmd/sync

test:
	go test ./...

build:
	go build -o bin/paipai-red-sync ./cmd/sync
