.PHONY: run test build frontend-dev frontend-build frontend-deploy postgres-readonly-deploy xhs-token xhs-refresh xhs-authd-build xhs-authd-start \
	lark-sync-build lark-sync-start lark-sync-manuscripts lark-sync-dandelion lark-sync-cid lark-sync-coenzyme-q10 lark-sync-status lark-sync-logs lark-sync-stop \
	lark-sync-cid-daily-install lark-sync-cid-daily-now lark-sync-cid-daily-status lark-sync-cid-daily-logs lark-sync-coenzyme-q10-daily-install lark-sync-coenzyme-q10-daily-now lark-sync-coenzyme-q10-daily-status lark-sync-coenzyme-q10-daily-logs \
	xhs-authd-authorize xhs-authd-status xhs-authd-logs xhs-authd-stop xhs-campaign-sync xhs-sync-status xhs-sync-campaigns xhs-sync-units xhs-sync-creativities \
	xhs-sync-daily xhs-sync-daily-install xhs-sync-daily-now xhs-sync-daily-status xhs-sync-daily-logs \
	embeddings-refresh embeddings-force-refresh guorai-build guorai-login guorai-sync guorai-sync-install guorai-sync-now guorai-sync-status guorai-sync-logs

run:
	@set -a; . ./.env; set +a; go run ./cmd/sync

lark-sync-build:
	go build -o bin/paipai-red-sync ./cmd/sync

lark-sync-start: lark-sync-build
	@set -e; set -a; . ./.env; set +a; \
	pm2 startOrReload ecosystem.config.cjs --only paipai-lark-sync --update-env; \
	ready=; \
	for attempt in $$(seq 1 60); do \
		schema_ready=$$(sudo -u postgres psql "$$DATABASE_URL" -X -Atqc "SELECT NOT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid='public.maituo_customer_daily_notes'::regclass AND attname IN ('subaccount','campaign_name') AND NOT attisdropped) AND EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='public.maituo_customer_daily_notes'::regclass AND contype='p' AND pg_get_constraintdef(oid)='PRIMARY KEY (report_date, note_id, placement)')" 2>/dev/null || true); \
		if curl -fsS http://127.0.0.1:18081/healthz >/dev/null && [ "$$schema_ready" = "t" ]; then ready=1; break; fi; \
		sleep 1; \
	done; \
	if [ -z "$$ready" ]; then echo "paipai-lark-sync or canonical Maituo schema did not become ready" >&2; exit 1; fi
	@$(MAKE) postgres-readonly-deploy

postgres-readonly-deploy:
	@set -e; set -a; . ./.env; set +a; \
	sudo -u postgres psql "$$DATABASE_URL" -X -v ON_ERROR_STOP=1 -f deploy/postgres/paipai_readonly.sql; \
	view_ready=$$(sudo -u postgres psql "$$DATABASE_URL" -X -Atqc "SELECT NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='paipai_readonly' AND table_name='maituo_customer_daily_notes' AND column_name IN ('subaccount','campaign_name')) AND POSITION('account_plan_archive' IN pg_get_viewdef('paipai_readonly.maituo_customer_daily_notes'::regclass, true))=0"); \
	if [ "$$view_ready" != "t" ]; then echo "Maituo readonly view did not bind to the canonical table" >&2; exit 1; fi

lark-sync-manuscripts:
	@curl -sS -X POST http://127.0.0.1:18081/v1/sync/manuscripts

lark-sync-dandelion:
	@curl -sS -X POST http://127.0.0.1:18081/v1/sync/dandelion

lark-sync-cid:
	@curl -sS -X POST http://127.0.0.1:18081/v1/sync/cid

lark-sync-coenzyme-q10: lark-sync-cid

lark-sync-cid-daily-install:
	systemd-analyze verify deploy/systemd/paipai-coenzyme-q10-sync.service deploy/systemd/paipai-coenzyme-q10-sync.timer
	@sudo install -m 0644 deploy/systemd/paipai-coenzyme-q10-sync.service /etc/systemd/system/paipai-coenzyme-q10-sync.service
	@sudo install -m 0644 deploy/systemd/paipai-coenzyme-q10-sync.timer /etc/systemd/system/paipai-coenzyme-q10-sync.timer
	@sudo systemctl daemon-reload
	@sudo systemctl enable --now paipai-coenzyme-q10-sync.timer

lark-sync-coenzyme-q10-daily-install: lark-sync-cid-daily-install

lark-sync-cid-daily-now:
	@sudo systemctl start paipai-coenzyme-q10-sync.service

lark-sync-coenzyme-q10-daily-now: lark-sync-cid-daily-now

lark-sync-cid-daily-status:
	@systemctl status paipai-coenzyme-q10-sync.timer paipai-coenzyme-q10-sync.service --no-pager

lark-sync-coenzyme-q10-daily-status: lark-sync-cid-daily-status

lark-sync-cid-daily-logs:
	@journalctl -u paipai-coenzyme-q10-sync.service -n 100 --no-pager

lark-sync-coenzyme-q10-daily-logs: lark-sync-cid-daily-logs

lark-sync-status:
	@curl -sS http://127.0.0.1:18081/v1/sync/manuscripts/status

embeddings-refresh:
	@set -a; . ./.env; set +a; go run ./cmd/embed-notes

embeddings-force-refresh:
	@set -a; . ./.env; set +a; go run ./cmd/embed-notes --force

guorai-build:
	go build -o bin/paipai-guorai ./cmd/guorai

guorai-login: guorai-build
	@set -a; . ./.env; set +a; ./bin/paipai-guorai login

guorai-sync: guorai-build
	@set -a; . ./.env; set +a; ./bin/paipai-guorai sync --type all --days 1 --note-window-days 14 --plan-window-days 14 --timeout 30m

guorai-sync-install: guorai-build
	systemd-analyze verify deploy/systemd/paipai-guorai-sync.service deploy/systemd/paipai-guorai-sync.timer
	@sudo install -m 0644 deploy/systemd/paipai-guorai-sync.service /etc/systemd/system/paipai-guorai-sync.service
	@sudo install -m 0644 deploy/systemd/paipai-guorai-sync.timer /etc/systemd/system/paipai-guorai-sync.timer
	@sudo systemctl daemon-reload
	@sudo systemctl enable --now paipai-guorai-sync.timer

guorai-sync-now: guorai-build
	@sudo systemctl start paipai-guorai-sync.service

guorai-sync-status:
	@systemctl status paipai-guorai-sync.timer paipai-guorai-sync.service --no-pager

guorai-sync-logs:
	@journalctl -u paipai-guorai-sync.service -n 100 --no-pager

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

xhs-sync-daily: xhs-authd-build
	@./bin/xhs-jg-authd sync-daily

xhs-sync-daily-install: xhs-authd-build
	systemd-analyze verify deploy/systemd/paipai-xhs-jg-sync.service deploy/systemd/paipai-xhs-jg-sync.timer
	@sudo install -m 0644 deploy/systemd/paipai-xhs-jg-sync.service /etc/systemd/system/paipai-xhs-jg-sync.service
	@sudo install -m 0644 deploy/systemd/paipai-xhs-jg-sync.timer /etc/systemd/system/paipai-xhs-jg-sync.timer
	@sudo systemctl daemon-reload
	@sudo systemctl enable --now paipai-xhs-jg-sync.timer

xhs-sync-daily-now: xhs-authd-build
	@sudo systemctl start paipai-xhs-jg-sync.service

xhs-sync-daily-status:
	@systemctl status paipai-xhs-jg-sync.timer paipai-xhs-jg-sync.service --no-pager

xhs-sync-daily-logs:
	@journalctl -u paipai-xhs-jg-sync.service -n 100 --no-pager

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
