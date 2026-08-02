.PHONY: run test build frontend-dev frontend-build frontend-deploy xhs-token xhs-refresh xhs-authd-build xhs-authd-start \
	lark-sync-build lark-sync-start lark-sync-manuscripts lark-sync-dandelion lark-sync-status lark-sync-logs lark-sync-stop \
	xhs-authd-authorize xhs-authd-status xhs-authd-logs xhs-authd-stop xhs-campaign-sync xhs-sync-status xhs-sync-campaigns xhs-sync-units xhs-sync-creativities \
	embeddings-refresh embeddings-force-refresh

run:
	@set -a; . ./.env; set +a; go run ./cmd/sync

lark-sync-build:
	go build -o bin/paipai-red-sync ./cmd/sync

lark-sync-start: lark-sync-build
	@set -a; . ./.env; set +a; pm2 startOrReload ecosystem.config.cjs --only paipai-lark-sync --update-env

lark-sync-manuscripts:
	@curl -sS -X POST http://127.0.0.1:18081/v1/sync/manuscripts

lark-sync-dandelion:
	@curl -sS -X POST http://127.0.0.1:18081/v1/sync/dandelion

lark-sync-status:
	@curl -sS http://127.0.0.1:18081/v1/sync/manuscripts/status

embeddings-refresh:
	@set -a; . ./.env; set +a; go run ./cmd/embed-notes

embeddings-force-refresh:
	@set -a; . ./.env; set +a; go run ./cmd/embed-notes --force

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

frontend-dev:
	npm run dev --prefix frontend

frontend-build:
	npm run build --prefix frontend

frontend-deploy: frontend-build
	@sudo install -d -o www-data -g www-data -m 0755 /var/www/paipai
	@sudo rsync -a --delete --chown=www-data:www-data frontend/dist/ /var/www/paipai/
	@sudo install -m 0644 deploy/nginx/paipai-console.conf /etc/nginx/snippets/paipai-console.conf
	@sudo nginx -t
	@sudo systemctl reload nginx
