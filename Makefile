.PHONY: run test build xhs-token xhs-refresh xhs-authd-build xhs-authd-start \
	xhs-authd-authorize xhs-authd-status xhs-authd-logs xhs-authd-stop xhs-campaign-sync xhs-sync-status xhs-sync-campaigns xhs-sync-units xhs-sync-creativities

run:
	@set -a; . ./.env; set +a; go run ./cmd/sync

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
