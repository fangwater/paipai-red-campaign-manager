.PHONY: run test build xhs-token xhs-refresh xhs-authd-build xhs-authd-start \
	lark-sync-build lark-sync-start lark-sync-manuscripts lark-sync-base lark-sync-status lark-sync-logs lark-sync-stop \
	xhs-authd-authorize xhs-authd-status xhs-authd-logs xhs-authd-stop xhs-campaign-sync xhs-sync-status xhs-sync-campaigns xhs-sync-units xhs-sync-creativities

run:
	@set -a; . ./.env; set +a; go run ./cmd/sync

lark-sync-build:
	go build -o bin/paipai-red-sync ./cmd/sync

lark-sync-start: lark-sync-build
	@set -a; . ./.env; set +a; pm2 startOrReload ecosystem.config.cjs --only paipai-lark-sync --update-env

lark-sync-manuscripts:
	@curl -sS -X POST http://127.0.0.1:18081/v1/sync/manuscripts

lark-sync-base:
	@curl -sS -X POST http://127.0.0.1:18081/v1/sync/base

lark-sync-status:
	@curl -sS http://127.0.0.1:18081/v1/sync/manuscripts/status

lark-sync-logs:
	pm2 logs paipai-lark-sync --lines 100

lark-sync-stop:
	pm2 stop paipai-lark-sync

xhs-token:
	@set -a; . ./.env; set +a; go run ./cmd/xhs-jg token

xhs-refresh:
	@set -a; . ./.env; set +a; go run ./cmd/xhs-jg refresh

xhs-authd-build:
	go build -o bin/xhs-jg-authd ./cmd/xhs-jg-authd

xhs-authd-start: xhs-authd-build
	@set -a; . ./.env; set +a; pm2 startOrReload ecosystem.config.cjs --only paipai-xhs-jg-authd --update-env

xhs-authd-authorize: xhs-authd-build
	@set -a; . ./.env; set +a; ./bin/xhs-jg-authd authorize

xhs-authd-status: xhs-authd-build
	@./bin/xhs-jg-authd status

xhs-sync-status: xhs-authd-build
	@set -a; . ./.env; set +a; ./bin/xhs-jg-authd sync-status

xhs-sync-campaigns: xhs-authd-build
	@set -a; . ./.env; set +a; ./bin/xhs-jg-authd sync-campaigns

xhs-sync-units: xhs-authd-build
	@set -a; . ./.env; set +a; ./bin/xhs-jg-authd sync-units

xhs-sync-creativities: xhs-authd-build
	@set -a; . ./.env; set +a; ./bin/xhs-jg-authd sync-creativities

xhs-authd-logs:
	pm2 logs paipai-xhs-jg-authd --lines 100

xhs-authd-stop:
	pm2 stop paipai-xhs-jg-authd

xhs-campaign-sync:
	@set -a; . ./.env; set +a; go run ./cmd/xhs-jg-campaign-sync

test:
	go test ./...

build:
	go build -o bin/paipai-red-sync ./cmd/sync
